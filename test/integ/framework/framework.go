// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package framework

import (
	"flag"

	"math/rand"
	"time"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/test/integ/client"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// Before represents constant for before
	Before = "before"
	// After represents constant for after
	After = "after"
)

// BackupStorageType type of storage
type BackupStorageType string

// Global framework.
var Global *Framework

// Framework handles communication with the kube cluster in e2e tests.
type Framework struct {
	KubeClient               kubernetes.Interface
	KubeClient2              kubernetes.Clientset
	KubeClientExt            clientset.Clientset
	RestClient               rest.RESTClient
	ExternalIP               string
	CRClient                 client.VMOCR
	Namespace                string
	OperatorNamespace        string
	SkipTeardown             bool
	RunID                    string
	Phase                    string
	IngressControllerSvcName string
	Ingress                  bool
	ElasticsearchVersion     string
}

// Setup sets up a test framework and initialises framework.Global.
func Setup() error {
	kubeconfig := flag.String("kubeconfig", "", "Path to kubeconfig file with authorization and master location information.")
	externalIP := flag.String("externalip", "localhost", "External IP over which to access deployments.")
	namespace := flag.String("namespace", "default", "Integration test namespace")
	operatorNamespace := flag.String("operatorNamespace", "", "Local test run mimicks prod environments")
	skipTeardown := flag.Bool("skipteardown", false, "Skips tearing down VMO instances created by the tests")
	runid := flag.String("runid", "test-"+generateRandomID(3), "Optional string that will be used to uniquely identify this test run.")
	phase := flag.String("phase", "", "Optional 'phase' to test ("+Before+", "+After+")")
	ingressControllerSvcName := flag.String("ingressControllerSvcName", "vmi-ingress-controller", "Ingress controller service name")
	ingress := flag.Bool("ingress", false, "Use ingress port for testing")
	flag.Parse()

	if *operatorNamespace == "" {
		operatorNamespace = namespace
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	crClient, err := client.NewCRClient(cfg)
	if err != nil {
		return err
	}

	kubeClientExt, err := clientset.NewForConfig(cfg)
	if err != nil {
		return err
	}

	s := schema.GroupVersion{Group: constants.VMOGroup, Version: constants.VMOVersion}
	cfg.GroupVersion = &s
	cfg.APIPath = "/apis"
	cfg.ContentType = runtime.ContentTypeJSON
	cfg.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: serializer.NewCodecFactory(&runtime.Scheme{})}
	restClient, err := rest.RESTClientFor(cfg)
	if err != nil {
		return err
	}

	Global = &Framework{
		KubeClient:               kubeClient,
		KubeClient2:              *kubeClient,
		KubeClientExt:            *kubeClientExt,
		RestClient:               *restClient,
		ExternalIP:               *externalIP,
		CRClient:                 crClient,
		Namespace:                *namespace,
		OperatorNamespace:        *operatorNamespace,
		SkipTeardown:             *skipTeardown,
		RunID:                    *runid,
		Phase:                    *phase,
		IngressControllerSvcName: *ingressControllerSvcName,
		Ingress:                  *ingress,
		ElasticsearchVersion:     "7.5.2",
	}

	return nil
}

// Teardown shuts down the test framework and cleans up.
func Teardown() error {
	Global = nil
	return nil
}

func generateRandomID(n int) string {
	rand.Seed(time.Now().Unix())
	var letter = []rune("abcdefghijklmnopqrstuvwxyz")

	id := make([]rune, n)
	for i := range id {
		id[i] = letter[rand.Intn(len(letter))]
	}
	return string(id)
}
