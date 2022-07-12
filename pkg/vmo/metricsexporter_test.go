// Copyright (c) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	vmctl "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/client/clientset/versioned"
	v1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/client/clientset/versioned/typed/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/configmaps"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/upgrade"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	fake "k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
)

// simple ClusterRoleLister implementation
type simpleClusterRoleLister struct {
	kubeClient kubernetes.Interface
}

// TestCreateCertificates tests that the certificates needed for webhooks are created
// GIVEN an output directory for certificates
//  WHEN I call CreateCertificates
//  THEN all the needed certificate artifacts are created
func TestNoMetrics(t *testing.T) {
	clearMetrics()
	assert := assert.New(t)
	registerMetricsHandlers()
	assert.Equal(0, len(allMetrics), "allMetrics array is not empty")
	assert.Equal(0, len(failedMetrics), "failedMetrics array is not empty")
}

func TestOneValidMetric(t *testing.T) {
	clearMetrics()
	assert := assert.New(t)
	firstValidMetric := prometheus.NewCounter(prometheus.CounterOpts{Name: "testOneValidMetric_A", Help: "This is the first valid metric"})
	allMetrics = append(allMetrics, firstValidMetric)
	registerMetricsHandlers()
	assert.Equal(1, len(allMetrics), "allMetrics array does not contain the one valid metric")
	assert.Equal(0, len(failedMetrics), "The valid metric failed")
}

func TestOneInvalidMetric(t *testing.T) {
	clearMetrics()
	assert := assert.New(t)
	firstInvalidMetric := prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the first invalid metric"})
	allMetrics = append(allMetrics, firstInvalidMetric)
	go registerMetricsHandlers()
	time.Sleep(time.Second * 1)
	assert.Equal(1, len(allMetrics), "allMetrics array does not contain the one invalid metric")
	assert.Equal(1, len(failedMetrics), "The invalid metric did not fail properly and was not retried")
}

func TestTwoValidMetrics(t *testing.T) {
	clearMetrics()
	assert := assert.New(t)
	firstValidMetric := prometheus.NewCounter(prometheus.CounterOpts{Name: "TestTwoValidMetrics_A", Help: "This is the first valid metric"})
	secondValidMetric := prometheus.NewCounter(prometheus.CounterOpts{Name: "TestTwoValidMetrics_B", Help: "This is the second valid metric"})
	allMetrics = append(allMetrics, firstValidMetric, secondValidMetric)
	registerMetricsHandlers()
	assert.Equal(2, len(allMetrics), "allMetrics array does not contain both valid metrics")
	assert.Equal(0, len(failedMetrics), "Some metrics failed")
}

func TestTwoInvalidMetrics(t *testing.T) {
	clearMetrics()
	assert := assert.New(t)
	firstInvalidMetric := prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the first invalid metric"})
	secondInvalidMetric := prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the second invalid metric"})
	allMetrics = append(allMetrics, firstInvalidMetric, secondInvalidMetric)
	go registerMetricsHandlers()
	time.Sleep(time.Second)
	assert.Equal(2, len(failedMetrics), "Both Invalid")
}

func TestThreeValidMetrics(t *testing.T) {
	clearMetrics()
	assert := assert.New(t)
	firstValidMetric := prometheus.NewCounter(prometheus.CounterOpts{Name: "TestThreeValidMetrics_A", Help: "This is the first valid metric"})
	secondValidMetric := prometheus.NewCounter(prometheus.CounterOpts{Name: "TestThreeValidMetrics_B", Help: "This is the second valid metric"})
	thirdValidMetric := prometheus.NewCounter(prometheus.CounterOpts{Name: "TestThreeValidMetrics_C", Help: "This is the third valid metric"})
	allMetrics = append(allMetrics, firstValidMetric, secondValidMetric, thirdValidMetric)
	registerMetricsHandlers()
	assert.Equal(3, len(allMetrics), "allMetrics array does not contain all metrics")
	assert.Equal(0, len(failedMetrics), "Some metrics failed")
}

func TestThreeInvalidMetrics(t *testing.T) {
	clearMetrics()
	assert := assert.New(t)
	firstInvalidMetric := prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the first invalid metric"})
	secondInvalidMetric := prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the second invalid metric"})
	thirdInvalidMetric := prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the third invalid metric"})
	allMetrics = append(allMetrics, firstInvalidMetric, secondInvalidMetric, thirdInvalidMetric)
	go registerMetricsHandlers()
	time.Sleep(time.Second)
	assert.Equal(3, len(failedMetrics), "All 3 invalid")
}

func TestReconcileMetrics(t *testing.T) {
	// vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	// operatorConfig := &config.OperatorConfig{}
	// expected, err := vmo
	const configMapName = "myDatasourcesConfigMap"

	// GIVEN a Grafana datasources configmap exists and the Prometheus URL is the legacy URL
	//  WHEN we call the createUpdateDatasourcesConfigMap
	//  THEN the configmap is updated and the Prometheus URL points to the new Prometheus instance
	vmo := &vmctl.VerrazzanoMonitoringInstance{}
	vmo.Name = constants.VMODefaultName
	vmo.Namespace = constants.VerrazzanoSystemNamespace

	// set the Prometheus URL to the legacy URL
	replaceMap := map[string]string{constants.GrafanaTmplPrometheusURI: resources.GetMetaName(vmo.Name, config.Prometheus.Name),
		constants.GrafanaTmplAlertManagerURI: ""}
	dataSourceTemplate, err := asDashboardTemplate(constants.DataSourcesTmpl, replaceMap)
	assert.NoError(t, err)

	cm := configmaps.NewConfig(vmo, configMapName, map[string]string{datasourceYAMLKey: dataSourceTemplate})

	client := fake.NewSimpleClientset(cm)
	defaultReplicasNum := 0
	vmo.Labels = make(map[string]string)
	//versioned.Interface{}
	//vmoclientset := fake.NewSimpleClientset()
	vmoclientset := versioned.Clientset{
		DiscoveryClient: &discovery.DiscoveryClient{},
		verrazzanoV1: fake.,
	}

	//controller, err := NewController("verrazzano-install", "", "v1", "default", "default", "", "")
	controller := &Controller{
		kubeclientset:   client,
		configMapLister: &simpleConfigMapLister{kubeClient: client},
		secretLister:    &simpleSecretLister{kubeClient: client},
		log:             vzlog.DefaultLogger(),
		operatorConfig: &config.OperatorConfig{
			EnvName:                        "",
			DefaultIngressTargetDNSName:    "",
			DefaultSimpleComponentReplicas: &defaultReplicasNum,
			MetricsPort:                    &defaultReplicasNum,
			NatGatewayIPs:                  []string{},
			Pvcs: config.Pvcs{
				StorageClass:   "",
				ZoneMatchLabel: "",
			},
		},
		indexUpgradeMonitor: &upgrade.Monitor{},
		clusterRoleLister:   kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Rbac().V1().ClusterRoles().Lister(),
		serviceLister:       kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Core().V1().Services().Lister(),
		storageClassLister:  kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Storage().V1().StorageClasses().Lister(),
		nodeLister:          kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Core().V1().Nodes().Lister(),
		deploymentLister:    kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Apps().V1().Deployments().Lister(),
		pvcLister:           kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Core().V1().PersistentVolumeClaims().Lister(),
		statefulSetLister:   kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Apps().V1().StatefulSets().Lister(),
		ingressLister:       kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Networking().V1().Ingresses().Lister(),
		vmoclientset:        vmoclientset,
	}
	err = createUpdateDatasourcesConfigMap(controller, vmo, configMapName, map[string]string{})
	assert.NoError(t, err)

	// fetch the configmap, the Prometheus URL should now be the new Prometheus URL
	cm, err = client.CoreV1().ConfigMaps(vmo.Namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})
	assert.NoError(t, err)

	previousReconcileCount := testutil.ToFloat64(reconcileCounter)
	controller.syncHandlerStandardMode(vmo)
	assert.Equal(t, previousReconcileCount, testutil.ToFloat64(reconcileCounter)-1)
}

// func newVMOReconciler(c client.Client)  {
// 	return vmo.NewController("verrazzano-install",)
// }

// func newScheme() *runtime.Scheme {
// 	scheme := runtime.NewScheme()
// 	_ = vmcontrollerv1.AddToScheme(scheme)
// 	_ = corev1.AddToScheme(scheme)
// 	return scheme
// }

func newRequest() ctrl.Request {
	return ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: "verrazzano-install",
			Name:      "default"}}
}

func clearMetrics() {
	allMetrics = []prometheus.Collector{}
	for c := range failedMetrics {
		delete(failedMetrics, c)
	}
	time.Sleep(time.Second * 1)
}

// func TestMain(t *testing.T) {
// 	clearMetrics()
// 	TestNoMetrics(t)
// 	TestOneValidMetric(t)
// 	TestOneInvalidMetric(t)
// 	TestTwoValidMetrics(t)
// 	TestTwoInvalidMetrics(t)
// 	TestThreeValidMetrics(t)
// 	TestThreeInvalidMetrics(t)
// }
