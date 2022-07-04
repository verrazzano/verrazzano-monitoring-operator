// Copyright (c) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package main

import (
	"flag"
	"fmt"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/log"
	"github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/opensearch"
	model "github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/types"
	futil "github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/utilities"
	kutil "github.com/verrazzano/verrazzano-monitoring-operator/verrazzano-backup-hook/utilities/k8s"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strings"
	"time"
)

var (
	VeleroBackupName string
	Component        string
	Operation        string
	Profile          string
)

func main() {
	flag.StringVar(&VeleroBackupName, "velero-backup-name", "", "The Velero-backup-name associated with this operation.")
	flag.StringVar(&Component, "component", "opensearch", "The Verrazzano component to be backed up or restored (Default = opensearch).")
	flag.StringVar(&Operation, "operation", "", "The operation to be performed - backup/restore.")
	flag.StringVar(&Profile, "profile", "default", "Object store credentials profile")

	// Add the zap logger flag set to the CLI.
	opts := kzap.Options{}
	opts.BindFlags(flag.CommandLine)

	flag.Parse()
	kzap.UseFlagOptions(&opts)

	// Flag validations
	if Operation == "" {
		fmt.Printf("Operation cannot be empty . It has to be 'backup/restore\n")
		os.Exit(1)
	}
	if Operation != constants.BackupOperation && Operation != constants.RestoreOperation {
		fmt.Printf("Operation has to be 'backup/restore\n")
		os.Exit(1)
	}
	if VeleroBackupName == "" {
		fmt.Printf("VeleroBackupName cannot be empty . It has to be set to an existing velero backup.\n")
		os.Exit(1)
	}

	//// Auto detect component based on injection
	//componentFound, err := futil.GetComponent(constants.ComponentPath)
	//if err != nil {
	//	fmt.Printf("Component detection failure %v", err)
	//	os.Exit(1)
	//}
	//Component = componentFound

	// Initialize the zap log
	file, err := os.CreateTemp(os.TempDir(), fmt.Sprintf("verrazzano-%s-hook-*.log", strings.ToLower(Operation)))
	if err != nil {
		fmt.Printf("Unable to create temp file")
		os.Exit(1)
	}
	defer file.Close()
	log, err := log.Logger(file.Name())
	if err != nil {
		fmt.Printf("Unable to fetch logger")
		os.Exit(1)
	}
	log.Info("Verrazzano backup and restore helper invoked.")

	// Gathering k8s clients
	done := false
	retryCount := 0
	k8sContextReady := true
	var config *rest.Config
	var kubeClientInterface *kubernetes.Clientset
	var kubeClient client.Client
	var dynamicKubeClientInterface dynamic.Interface

	globalTimeout := futil.GetEnvWithDefault(constants.OpenSearchHealthCheckTimeoutKey, constants.OpenSearchHealthCheckTimeoutDefaultValue)
	globalTimeoutDuration, err := time.ParseDuration(globalTimeout)
	if err != nil {
		log.Errorf("Unable to parse time duration ", zap.Error(err))
		os.Exit(1)
	}

	var checkConData model.ConnectionData
	// Initialize Opensearch object
	search := opensearch.New(constants.OpenSearchURL, globalTimeoutDuration, http.DefaultClient, &checkConData, log)
	// Check OpenSearch health before proceeding with backup or restore
	err = search.EnsureOpenSearchIsHealthy()
	if err != nil {
		log.Errorf("Operation cannot be performed as OpenSearch is not healthy")
		os.Exit(1)
	}

	// Feedback loop to gather k8s context
	for !done {
		config, err = ctrl.GetConfig()
		if err != nil {
			log.Errorf("Failed to get kubeconfig: %v", err)
			k8sContextReady = false
		}
		kubeClientInterface, err = kubernetes.NewForConfig(config)
		if err != nil {
			log.Errorf("Failed to get clientset: %v", err)
			k8sContextReady = false
		}
		kubeClient, err = client.New(config, client.Options{})
		if err != nil {
			log.Errorf("Failed to get controller-runtime client: %v", err)
			k8sContextReady = false
		}
		dynamicKubeClientInterface, err = dynamic.NewForConfig(config)
		if err != nil {
			log.Errorf("Failed to get dynamic client: %v", err)
			k8sContextReady = false
		}

		if !k8sContextReady {
			if retryCount <= constants.RetryCount {
				message := "Unable to get context"
				_, err := futil.WaitRandom(message, globalTimeout, log)
				if err != nil {
					log.Panic(err)
				}
				retryCount = retryCount + 1
			}
		} else {
			done = true
			log.Info("kubecontext retrieval successful")
		}

	}

	// Initialize K8s object
	k8s := kutil.New(dynamicKubeClientInterface, kubeClient, kubeClientInterface, config, Profile, log)

	// Get S3 access details from Velero Backup Storage location associated with Backup given as input
	// Ensure the Backup Storage Location is NOT default
	conData, err := k8s.PopulateConnData(constants.VeleroNameSpace, VeleroBackupName)
	if err != nil {
		log.Errorf("Unable to fetch secret: %v", err)
		os.Exit(1)
	}

	/*
		// Initialize Opensearch object
		search := opensearch.New(constants.OpenSearchURL, globalTimeoutDuration, http.DefaultClient, conData, log)
		// Check OpenSearch health before proceeding with backup or restore
		err = search.EnsureOpenSearchIsHealthy()
		if err != nil {
			log.Errorf("Operation cannot be performed as OpenSearch is not healthy")
			os.Exit(1)
		}
	*/

	// Update OpenSearch keystore
	_, err = k8s.UpdateKeystore(conData)
	if err != nil {
		log.Errorf("Unable to update keystore")
		os.Exit(1)
	}

	err = search.ReloadOpensearchSecureSettings()
	if err != nil {
		log.Errorf("Unable to reload security settings")
		os.Exit(1)
	}

	switch strings.ToLower(Operation) {
	// OpenSearch backup handling
	case constants.BackupOperation:
		log.Info("Commencing opensearch backup ..")
		err = search.Backup()
		if err != nil {
			log.Errorf("Operation '%s' unsuccessfull due to %v", Operation, zap.Error(err))
			os.Exit(1)
		}
		log.Infof("%s backup was successfull", strings.ToTitle(Component))

	case constants.RestoreOperation:
		// OpenSearch restore handling
		log.Infof("Commencing OpenSearch restore ..")
		err = k8s.ScaleDeployment(constants.VMOLabelSelector, constants.VerrazzanoNameSpaceName, constants.VMODeploymentName, int32(0))
		if err != nil {
			log.Errorf("Unable to scale deployment '%s' due to %v", constants.VMODeploymentName, zap.Error(err))
			os.Exit(1)
		}
		err = k8s.ScaleDeployment(constants.IngestLabelSelector, constants.VerrazzanoNameSpaceName, constants.IngestDeploymentName, int32(0))
		if err != nil {
			log.Errorf("Unable to scale deployment '%s' due to %v", constants.IngestDeploymentName, zap.Error(err))
			os.Exit(1)
		}
		err = search.Restore()
		if err != nil {
			log.Errorf("Operation '%s' unsuccessfull due to %v", Operation, zap.Error(err))
			os.Exit(1)
		}

		ok, err := k8s.CheckDeployment(constants.KibanaDeploymentLabelSelector, constants.VerrazzanoNameSpaceName)
		if err != nil {
			log.Errorf("Unable to detect Kibana deployment '%s' due to %v", constants.KibanaDeploymentLabelSelector, zap.Error(err))
			os.Exit(1)
		}
		// If kibana is deployed then scale it down
		if ok {
			err = k8s.ScaleDeployment(constants.KibanaLabelSelector, constants.VerrazzanoNameSpaceName, constants.KibanaDeploymentName, int32(0))
			if err != nil {
				log.Errorf("Unable to scale deployment '%s' due to %v", constants.IngestDeploymentName, zap.Error(err))
			}
		}
		err = k8s.ScaleDeployment(constants.VMOLabelSelector, constants.VerrazzanoNameSpaceName, constants.VMODeploymentName, int32(1))
		if err != nil {
			log.Errorf("Unable to scale deployment '%s' due to %v", constants.VMODeploymentName, zap.Error(err))
		}

		err = k8s.CheckAllPodsAfterRestore()
		if err != nil {
			log.Errorf("Unable to check deployments after restoring Verrazzano Monitoring Operator %v", zap.Error(err))
		}

		log.Infof("%s restore was successfull", strings.ToTitle(Component))

	}

}
