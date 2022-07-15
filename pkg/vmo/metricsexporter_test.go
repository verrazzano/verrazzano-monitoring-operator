// Copyright (c) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	vmctl "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	vmofake "github.com/verrazzano/verrazzano-monitoring-operator/pkg/client/clientset/versioned/fake"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/configmaps"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/upgrade"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	kubeinformers "k8s.io/client-go/informers"
	fake "k8s.io/client-go/kubernetes/fake"
)

// TestInitializeAllMetricsArray tests that the metrics maps are added to the allmetrics array
// GIVEN populated metrics maps
//  WHEN I call initializeAllMetricsArray
//  THEN all the needed metrics are placed in the allmetrics array
func TestInitializeAllMetricsArray(t *testing.T) {
	clearMetrics()
	assert := assert.New(t)
	initializeAllMetricsArray()
	assert.Equal(14, len(allMetrics), "There may be new metrics in the map, or some metrics may not be added to the allmetrics array from the metrics maps")
	//This number should correspond to the number of total metrics, including metrics inside of metric maps
}

// TestNoMetrics, TestValid & TestInvalid tests that metrics in the allmetrics array are registered and failedMetrics are retried
// GIVEN a populated allMetrics array
//  WHEN I call registerMetricsHandlers
//  THEN all the valid metrics are registered and failedMetrics are retried
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
	assert.Equal(3, len(failedMetrics), "Both Invalid")
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

func createControllerForTesting() (*Controller, *vmctl.VerrazzanoMonitoringInstance) {
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
	dataSourceTemplate, _ := asDashboardTemplate(constants.DataSourcesTmpl, replaceMap)

	cm := configmaps.NewConfig(vmo, configMapName, map[string]string{datasourceYAMLKey: dataSourceTemplate})

	client := fake.NewSimpleClientset(cm)
	defaultReplicasNum := 0
	vmo.Labels = make(map[string]string)
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
		vmoclientset:        vmofake.NewSimpleClientset(),
	}
	_ = createUpdateDatasourcesConfigMap(controller, vmo, configMapName, map[string]string{})

	return controller, vmo
}

// TestReconcileMetrics tests that the FunctionMetrics methods record metrics properly when the reconcile function is called
// GIVEN a FunctionMetric corresponding to the reconcile function
//  WHEN I call reconcile
//  THEN the metrics for the reconcile function are to be captured
func TestReconcileMetrics(t *testing.T) {
	controller, vmo := createControllerForTesting()

	DefaultLabelFunction = func(idx int64) string { return "1" }
	previousCount := testutil.ToFloat64(FunctionMetricsMap["reconcile"].callsTotal.metric)

	controller.syncHandlerStandardMode(vmo)

	newTimeStamp := testutil.ToFloat64(FunctionMetricsMap["reconcile"].lastCallTimestamp.metric.WithLabelValues("1"))
	newErrorCount := testutil.ToFloat64(FunctionMetricsMap["reconcile"].errorTotal.metric.WithLabelValues("1"))
	newCount := testutil.ToFloat64(FunctionMetricsMap["reconcile"].callsTotal.metric)

	assert.Equal(t, previousCount, float64(newCount-1))
	assert.Equal(t, newErrorCount, float64(1))
	assert.LessOrEqual(t, int64(newTimeStamp*10)/10, time.Now().Unix())
}

// TestDeploymentMetrics tests that the FunctionMetrics methods record metrics properly when the createDeployment function is called
// GIVEN a FunctionMetric corresponding to the deployment function
//  WHEN I call createDeployments
//  THEN the metrics for the CreateDeployments function are to be captured, with the exception of (trivial) error metrics
func TestDeploymentMetrics(t *testing.T) {
	controller, vmo := createControllerForTesting()

	DefaultLabelFunction = func(idx int64) string { return "1" }
	previousCount := testutil.ToFloat64(FunctionMetricsMap["deployment"].callsTotal.metric)

	CreateDeployments(controller, vmo, map[string]string{}, true)

	newTimeStamp := testutil.ToFloat64(FunctionMetricsMap["deployment"].lastCallTimestamp.metric.WithLabelValues("1"))
	newCount := testutil.ToFloat64(FunctionMetricsMap["deployment"].callsTotal.metric)
	//The error is incremented outside of the deployment function, it is quite trivial

	assert.Equal(t, previousCount, float64(newCount-1))
	assert.LessOrEqual(t, int64(newTimeStamp*10)/10, time.Now().Unix())
}

//helper function to ensure consistency between tests
func clearMetrics() {
	allMetrics = []prometheus.Collector{}
	for c := range failedMetrics {
		delete(failedMetrics, c)
	}
	time.Sleep(time.Second * 1)
}
