// Copyright (c) 2020, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	vmctl "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	vmofake "github.com/verrazzano/verrazzano-monitoring-operator/pkg/client/clientset/versioned/fake"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/metricsexporter"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/opensearch"
	dashboards "github.com/verrazzano/verrazzano-monitoring-operator/pkg/opensearch_dashboards"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/configmaps"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/upgrade"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
)

type registerTest struct {
	name               string
	allMetricsLength   int
	failedMetricLength int
	allMetrics         []prometheus.Collector
}

var allMetrics = metricsexporter.TestDelegate.GetAllMetricsArray()
var delegate = metricsexporter.TestDelegate

// TestInitializeAllMetricsArray tests that the metrics maps are added to the allmetrics array
// GIVEN populated metrics maps
//
//	WHEN I call initializeAllMetricsArray
//	THEN all the needed metrics are placed in the allmetrics array
func TestInitializeAllMetricsArray(t *testing.T) {
	clearMetrics()
	assert := assert.New(t)
	metricsexporter.TestDelegate.InitializeAllMetricsArray()
	//This number should correspond to the number of total metrics, including metrics inside of metric maps
	assert.Equal(30, len(*allMetrics), "There may be new metrics in the map, or some metrics may not be added to the allmetrics array from the metrics maps")
}

// TestNoMetrics, TestValid & TestInvalid tests that metrics in the allmetrics array are registered and failedMetrics are retried
// GIVEN a populated allMetrics array
//
//	WHEN I call registerMetricsHandlers
//	THEN all the valid metrics are registered and failedMetrics are retried
func TestRegistrationSystem(t *testing.T) {
	testCases := []registerTest{
		{
			name:               "TestNoMetrics",
			allMetrics:         []prometheus.Collector{},
			allMetricsLength:   0,
			failedMetricLength: 0,
		},
		{
			name: "TestOneValidMetric",
			allMetrics: []prometheus.Collector{
				prometheus.NewCounter(prometheus.CounterOpts{Name: "testOneValidMetric_A", Help: "This is the first valid metric"}),
			},
			allMetricsLength:   1,
			failedMetricLength: 0,
		},
		{
			name: "TestOneInvalidMetric",
			allMetrics: []prometheus.Collector{
				prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the first invalid metric"}),
			},
			allMetricsLength:   1,
			failedMetricLength: 1,
		},
		{
			name: "TestTwoValidMetrics",
			allMetrics: []prometheus.Collector{
				prometheus.NewCounter(prometheus.CounterOpts{Name: "TestTwoValidMetrics_A", Help: "This is the first valid metric"}),
				prometheus.NewCounter(prometheus.CounterOpts{Name: "TestTwoValidMetrics_B", Help: "This is the second valid metric"}),
			},
			allMetricsLength:   2,
			failedMetricLength: 0,
		},
		{
			name: "TestTwoInvalidMetrics",
			allMetrics: []prometheus.Collector{
				prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the first invalid metric"}),
				prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the second invalid metric"}),
			},
			allMetricsLength:   2,
			failedMetricLength: 2,
		},
		{
			name: "TestThreeValidMetrics",
			allMetrics: []prometheus.Collector{
				prometheus.NewCounter(prometheus.CounterOpts{Name: "TestThreeValidMetrics_A", Help: "This is the first valid metric"}),
				prometheus.NewCounter(prometheus.CounterOpts{Name: "TestThreeValidMetrics_B", Help: "This is the second valid metric"}),
				prometheus.NewCounter(prometheus.CounterOpts{Name: "TestThreeValidMetrics_C", Help: "This is the third valid metric"}),
			},
			allMetricsLength:   3,
			failedMetricLength: 0,
		},
		{
			name: "TestThreeInvalidMetrics",
			allMetrics: []prometheus.Collector{
				prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the first invalid metric"}),
				prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the second invalid metric"}),
				prometheus.NewCounter(prometheus.CounterOpts{Help: "This is the third invalid metric"}),
			},
			allMetricsLength:   3,
			failedMetricLength: 3,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			clearMetrics()
			assert := assert.New(t)
			*allMetrics = testCase.allMetrics
			metricsexporter.TestDelegate.RegisterMetricsHandlers(false)
			assert.Equal(testCase.allMetricsLength, len(*allMetrics), "allMetrics array length is not correct")
			assert.Equal(testCase.failedMetricLength, len(delegate.GetFailedMetricsMap()), "failedMetrics map length is not correct")
		})
	}
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

	cfg := &rest.Config{}
	kubeextclientset, _ := apiextensionsclient.NewForConfig(cfg)

	client := fake.NewSimpleClientset(cm)
	defaultReplicasNum := 0
	vmo.Labels = make(map[string]string)
	statefulSetLister := kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Apps().V1().StatefulSets().Lister()
	controller := &Controller{
		kubeclientset:    client,
		kubeextclientset: kubeextclientset,
		configMapLister:  &simpleConfigMapLister{kubeClient: client},
		secretLister:     &simpleSecretLister{kubeClient: client},
		log:              vzlog.DefaultLogger(),
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
		statefulSetLister:   statefulSetLister,
		ingressLister:       kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Networking().V1().Ingresses().Lister(),
		vmoclientset:        vmofake.NewSimpleClientset(),
		workqueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "VMOs"),
		osClient:            opensearch.NewOSClient(statefulSetLister),
		osDashboardsClient:  dashboards.NewOSDashboardsClient(),
	}
	_ = createUpdateDatasourcesConfigMap(controller, vmo, configMapName, map[string]string{})

	return controller, vmo
}

// TestReconcileMetrics tests that the FunctionMetrics methods record metrics properly when the reconcile function is called
// GIVEN a FunctionMetric corresponding to the reconcile function
//
//	WHEN I call reconcile
//	THEN the metrics for the reconcile function are to be captured
func TestReconcileAndUpdateMetrics(t *testing.T) {

	controller, vmo := createControllerForTesting()

	metricsexporter.DefaultLabelFunction = func(idx int64) string { return "1" }
	previousCount := testutil.ToFloat64(delegate.GetFunctionCounterMetric(metricsexporter.NamesReconcile))
	previousUpdateCount := testutil.ToFloat64(delegate.GetCounterMetric(metricsexporter.NamesVMOUpdate))

	controller.syncHandlerStandardMode(vmo)

	newTimeStamp := testutil.ToFloat64(delegate.GetFunctionTimestampMetric(metricsexporter.NamesReconcile).WithLabelValues("1"))
	newErrorCount := testutil.ToFloat64(delegate.GetFunctionErrorMetric(metricsexporter.NamesReconcile).WithLabelValues("1"))
	newCount := testutil.ToFloat64(delegate.GetFunctionCounterMetric(metricsexporter.NamesReconcile))
	newUpdateCount := testutil.ToFloat64(delegate.GetCounterMetric(metricsexporter.NamesVMOUpdate))

	assert.Equal(t, previousCount, float64(newCount-1))
	assert.Equal(t, newErrorCount, float64(1))
	assert.Equal(t, previousUpdateCount, float64(newUpdateCount-1))
	assert.LessOrEqual(t, int64(newTimeStamp*10)/10, time.Now().Unix())
}

// TestDeploymentMetrics tests that the FunctionMetrics methods record metrics properly when the createDeployment function is called
// GIVEN a FunctionMetric corresponding to the deployment function
//
//	WHEN I call createDeployments
//	THEN the metrics for the CreateDeployments function are to be captured, with the exception of (trivial) error metrics
func TestDeploymentMetrics(t *testing.T) {

	controller, vmo := createControllerForTesting()

	metricsexporter.DefaultLabelFunction = func(idx int64) string { return "1" }
	previousCount := testutil.ToFloat64(delegate.GetFunctionCounterMetric(metricsexporter.NamesDeployment))

	CreateDeployments(controller, vmo, map[string]string{}, true)

	newTimeStamp := testutil.ToFloat64(delegate.GetFunctionTimestampMetric(metricsexporter.NamesDeployment).WithLabelValues("1"))
	newCount := testutil.ToFloat64(delegate.GetFunctionCounterMetric(metricsexporter.NamesDeployment))
	//The error is incremented outside of the deployment function, it is quite trivial

	assert.Equal(t, previousCount, float64(newCount-1))
	assert.LessOrEqual(t, int64(newTimeStamp*10)/10, time.Now().Unix())
}

func TestIngressMetrics(t *testing.T) {
	controller, vmo := createControllerForTesting()
	metricsexporter.DefaultLabelFunction = func(idx int64) string { return "1" }
	previousCount := testutil.ToFloat64(delegate.GetFunctionCounterMetric(metricsexporter.NamesIngress))
	CreateIngresses(controller, vmo)
	newTimeStamp := testutil.ToFloat64(delegate.GetFunctionTimestampMetric(metricsexporter.NamesIngress).WithLabelValues("1"))
	newCount := testutil.ToFloat64(delegate.GetFunctionCounterMetric(metricsexporter.NamesIngress))
	assert.Equal(t, previousCount, float64(newCount-1))
	assert.LessOrEqual(t, int64(newTimeStamp*10)/10, time.Now().Unix())
}

func TestRoleBindingMetrics(t *testing.T) {
	controller, vmo := createControllerForTesting()
	previousCount := testutil.ToFloat64(delegate.GetCounterMetric(metricsexporter.NamesRoleBindings))
	CreateRoleBindings(controller, vmo)
	newCount := testutil.ToFloat64(delegate.GetCounterMetric(metricsexporter.NamesRoleBindings))
	assert.Equal(t, previousCount, float64(newCount-1))
}

func TestThreadingMetrics(t *testing.T) {
	controller, _ := createControllerForTesting()
	gauge := delegate.GetGaugeMetrics(metricsexporter.NamesQueue)
	gauge.Set(100)
	controller.IsHealthy()
	newCount := testutil.ToFloat64(gauge)
	assert.Equal(t, 0, int(newCount))
}

func TestConfigMapMetrics(t *testing.T) {
	controller, vmo := createControllerForTesting()
	previousCount := testutil.ToFloat64(delegate.GetCounterMetric(metricsexporter.NamesConfigMap))
	CreateConfigmaps(controller, vmo)
	newCount := testutil.ToFloat64(delegate.GetCounterMetric(metricsexporter.NamesConfigMap))
	newTimeStamp := testutil.ToFloat64(delegate.GetFunctionTimestampMetric(metricsexporter.NamesIngress).WithLabelValues(strconv.FormatInt(int64(previousCount)+1, 10)))
	assert.Equal(t, previousCount, float64(newCount-1))
	assert.LessOrEqual(t, int64(newTimeStamp*10)/10, time.Now().Unix())
}
func TestServiceMetrics(t *testing.T) {
	clearMetrics()
	controller, vmo := createControllerForTesting()
	previousCount := testutil.ToFloat64(delegate.GetCounterMetric(metricsexporter.NamesServices))
	CreateServices(controller, vmo)
	newCount := testutil.ToFloat64(delegate.GetCounterMetric(metricsexporter.NamesServices))
	newServicesCreated := testutil.ToFloat64(delegate.GetCounterMetric(metricsexporter.NamesServicesCreated))
	newTimeStamp := testutil.ToFloat64(delegate.GetTimestampMetric(metricsexporter.NamesServices).WithLabelValues(strconv.FormatInt(int64(previousCount)+1, 10)))
	assert.Equal(t, previousCount, float64(newCount-1))
	assert.EqualValues(t, 1, newServicesCreated)
	assert.LessOrEqual(t, int64(newTimeStamp*10)/10, time.Now().Unix())
}

// helper function to ensure consistency between tests
func clearMetrics() {
	*allMetrics = []prometheus.Collector{}
	for c := range metricsexporter.TestDelegate.GetFailedMetricsMap() {
		delete(metricsexporter.TestDelegate.GetFailedMetricsMap(), c) //maps are references, hence we can delete like normal here
	}
	time.Sleep(time.Second * 1)
	metricsexporter.RequiredInitialization()
}
