// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package main

import (
	"flag"
	"fmt"
	"os"

	kzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/vmo"
	"go.uber.org/zap"
)

var (
	masterURL      string
	kubeconfig     string
	namespace      string
	watchNamespace string
	watchVmi       string
	configmapName  string
	buildVersion   string
	buildDate      string
	certdir        string
	port           string
	zapOptions     = kzap.Options{}
)

func main() {
	flag.Parse()
	logs.InitLogs(zapOptions)

	// Namespace must still be specified on the command line, as it is a prerequisite to fetching the config map
	if namespace == "" {
		zap.S().Fatalf("A namespace must be specified")
	}

	// Initialize the images to use
	err := config.InitComponentDetails()
	if err != nil {
		zap.S().Fatalf("Error identifying docker images: %s", err.Error())
	}

	zap.S().Debugf("Creating new controller in namespace %s.", namespace)
	controller, err := vmo.NewController(namespace, configmapName, buildVersion, kubeconfig, masterURL, watchNamespace, watchVmi)
	if err != nil {
		zap.S().Fatalf("Error creating the controller: %s", err.Error())
	}

	_, err = vmo.CreateCertificates(certdir)
	if err != nil {
		zap.S().Fatalf("Error creating certificates: %s", err.Error())
		os.Exit(1)
	}

	vmo.StartHTTPServer(controller, certdir, port)

	if err = controller.Run(1); err != nil {
		zap.S().Fatalf("Error running controller: %s", err.Error())
	}
}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&namespace, "namespace", constants.DefaultNamespace, "The namespace in which this operator runs.")
	flag.StringVar(&watchNamespace, "watchNamespace", "", "Optionally, a namespace to watch exclusively.  If not set, all namespaces will be watched.")
	flag.StringVar(&watchVmi, "watchVmi", "", "Optionally, a specific VMI to watch exclusively.  If not set, all VMIs will be watched.")
	flag.StringVar(&configmapName, "configmapName", config.DefaultOperatorConfigmapName, "The configmap name containing the operator config")
	flag.StringVar(&certdir, "certdir", "/etc/certs", "the directory to initalize certificates into")
	flag.StringVar(&port, "port", "8080", "VMO server HTTP port")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s version %s\n", os.Args[0], buildVersion)
		fmt.Fprintf(os.Stderr, "built %s\n", buildDate)
		flag.PrintDefaults()
	}
	zapOptions.BindFlags(flag.CommandLine)
}
