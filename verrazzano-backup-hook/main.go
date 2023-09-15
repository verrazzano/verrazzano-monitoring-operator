// Copyright (c) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package main

import (
	"crypto/tls"
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
)

var (
	VeleroBackupName string
	Component        string
	Operation        string
	Profile          string
	VeleroNamespace  string
)

func main() {
	flag.StringVar(&VeleroBackupName, "velero-backup-name", "", "The Velero-backup-name associated with this operation.")
	flag.StringVar(&Component, "component", "opensearch", "The Verrazzano component to be backed up or restored (Default = opensearch).")
	flag.StringVar(&Operation, "operation", "", "Operation must be one of 'backup' or 'restore'.")
	flag.StringVar(&Profile, "profile", "default", "Object store credentials profile.")
	flag.StringVar(&VeleroNamespace, "namespace", "verrazzano-backup", "Namespace where Velero component is deployed.")

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
		fmt.Printf("VeleroBackupName must refer to an existing Velero backup.\n")
		os.Exit(1)
	}

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
	var checkConData model.ConnectionData
	checkConData.VeleroTimeout = globalTimeout

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
				// setting k8sContextReady flag to true os that subsequent checks dont re-use old value
				// This flag will be changed to false if any k8s go-client retrieval fails.
				log.Info("Resetting k8s context flag to true...")
				k8sContextReady = true
			}
		} else {
			done = true
			log.Info("kubecontext retrieval successful")
		}
	}

	// Initialize K8s object
	k8s := kutil.New(dynamicKubeClientInterface, kubeClient, kubeClientInterface, config, Profile, log)

	httpClient := http.DefaultClient
	opensearchURL := constants.OpenSearchURL
	isLegacyOS, err := k8s.IsLegacyOS()

	if err != nil {
		log.Errorf("Failed to determine if legacy OS is enabled: %v", err)
		os.Exit(1)
	}

	// Log which OS we are backing up or restoring
	if isLegacyOS {
		log.Infof("VMO OpenSearch found. Backup and Restore will be done for VMO OpenSearch")
	} else {
		log.Infof("Opster OpenSearch found. Backup and Restore will be done for Opster OpenSearch")
	}

	basicAuth := opensearch.NewBasicAuth(false, "", "")
	if !isLegacyOS {
		username, err := os.ReadFile("/mnt/admin-credentials/username")
		if err != nil {
			log.Errorf("Failed to get username for basic auth")
			os.Exit(1)
		}
		password, err := os.ReadFile("/mnt/admin-credentials/password")
		if err != nil {
			log.Errorf("Failed to get password for basic auth")
			os.Exit(1)
		}

		basicAuth = opensearch.NewBasicAuth(true, string(username), string(password))
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec //#nosec G402
		}
		httpClient = &http.Client{Transport: tr}
		opensearchURL = constants.OpenSearchHTTPSURL
	}

	// Initialize Opensearch object
	search := opensearch.New(opensearchURL, globalTimeout, httpClient, &checkConData, log, basicAuth)
	// Check OpenSearch health before proceeding with backup or restore
	err = search.EnsureOpenSearchIsHealthy()
	if err != nil {
		log.Errorf("Operation cannot be performed as OpenSearch is not healthy")
		os.Exit(1)
	}

	// Get S3 access details from Velero Backup Storage location associated with Backup given as input
	// Ensure the Backup Storage Location is NOT default
	openSearchConData, err := k8s.PopulateConnData(VeleroNamespace, VeleroBackupName)
	if err != nil {
		log.Errorf("Unable to fetch secret: %v", err)
		os.Exit(1)
	}

	// Update OpenSearch keystore
	_, err = k8s.UpdateKeystore(openSearchConData, globalTimeout)
	if err != nil {
		log.Errorf("Unable to update keystore")
		os.Exit(1)
	}

	openSearch := opensearch.New(opensearchURL, globalTimeout, httpClient, openSearchConData, log, basicAuth)
	err = search.ReloadOpensearchSecureSettings()
	if err != nil {
		log.Errorf("Unable to reload security settings")
		os.Exit(1)
	}

	switch strings.ToLower(Operation) {
	// OpenSearch backup handling
	case constants.BackupOperation:
		log.Info("Commencing opensearch backup ..")
		err = openSearch.Backup()
		if err != nil {
			log.Errorf("Operation '%s' unsuccessfull due to %v", Operation, zap.Error(err))
			os.Exit(1)
		}
		log.Infof("%s backup was successfull", strings.ToTitle(Component))

	case constants.RestoreOperation:
		// OpenSearch restore handling
		log.Infof("Commencing OpenSearch restore ..")
		operatorDeploymentName := constants.OpsterDeploymentName
		namespace := constants.VerrazzanoLoggingNamespace
		operatorDeploymentLabel := constants.OpsterDeploymentLabel

		if isLegacyOS {
			operatorDeploymentName = constants.VMODeploymentName
			operatorDeploymentLabel = constants.VMOLabelSelector
			namespace = constants.VerrazzanoSystemNamespace
		}
		err = k8s.ScaleDeployment(operatorDeploymentLabel, namespace, operatorDeploymentName, int32(0))
		if err != nil {
			log.Errorf("Unable to scale deployment '%s' due to %v", operatorDeploymentName, zap.Error(err))
			os.Exit(1)
		}
		if isLegacyOS {
			err = k8s.ScaleDeployment(constants.IngestLabelSelector, constants.VerrazzanoSystemNamespace, constants.IngestDeploymentName, int32(0))
			if err != nil {
				log.Errorf("Unable to scale deployment '%s' due to %v", constants.IngestDeploymentName, zap.Error(err))
				os.Exit(1)
			}
		} else {
			err = k8s.ScaleSTS(constants.NewIngestStatefulSetName, constants.VerrazzanoLoggingNamespace, int32(0))
			if err != nil {
				log.Errorf("Unable to scale sts '%s' due to %v", constants.NewIngestLabelSelector, zap.Error(err))
				os.Exit(1)
			}
		}

		if err != nil {
			log.Errorf("Unable to scale deployment '%s' due to %v", constants.IngestDeploymentName, zap.Error(err))
			os.Exit(1)
		}
		err = openSearch.Restore()
		if err != nil {
			log.Errorf("Operation '%s' unsuccessfull due to %v", Operation, zap.Error(err))
			os.Exit(1)
		}

		osdDeploymentName := constants.OSDDeploymentName
		osdLabel := constants.OSDDeploymentLabelSelector
		if isLegacyOS {
			osdDeploymentName = constants.KibanaDeploymentName
			osdLabel = constants.KibanaDeploymentLabelSelector
		}
		ok, err := k8s.CheckDeployment(osdLabel, namespace)
		if err != nil {
			log.Errorf("Unable to detect OSD deployment '%s' due to %v", osdLabel, zap.Error(err))
			os.Exit(1)
		}
		// If kibana is deployed then scale it down
		if ok {
			err = k8s.ScaleDeployment(osdLabel, namespace, osdDeploymentName, int32(0))
			if err != nil {
				log.Errorf("Unable to scale deployment '%s' due to %v", osdDeploymentName, zap.Error(err))
			}
		}
		err = k8s.ScaleDeployment(operatorDeploymentLabel, namespace, operatorDeploymentName, int32(1))
		if err != nil {
			log.Errorf("Unable to scale deployment '%s' due to %v", operatorDeploymentName, zap.Error(err))
		}

		err = k8s.CheckAllPodsAfterRestore()
		if err != nil {
			log.Errorf("Unable to check deployments after restoring Opensearch Operator %v", zap.Error(err))
		}

		log.Infof("%s restore was successfull", strings.ToTitle(Component))

	}

}
