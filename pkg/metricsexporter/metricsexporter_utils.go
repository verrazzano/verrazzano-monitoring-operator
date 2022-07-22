package metricsexporter

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

//This is intialized because adding the statement in the var block would create a cycle
func RequiredInitialization() {
	MetricsExp = metricsExporter{
		internalMetricsDelegate: metricsDelegate{},
		internalConfig:          initConfiguration(),
		internalData: data{
			functionMetricsMap:     initFunctionMetricsMap(),
			simpleCounterMetricMap: initSimpleCounterMetricMap(),
			simpleGaugeMetricMap:   initSimpleGaugeMetricMap(),
			durationMetricMap:      initDurationMetricMap(),
			timestampMetricMap:     initTimestampMetricMap(),
			errorMetricMap:         initErrorMetricMap(),
		},
	}

	DefaultLabelFunction = func(index int64) string { return numToString(index) }
	deploymentLabelFunction = MetricsExp.internalData.functionMetricsMap[NamesDeployment].GetLabel
	configMapLabelFunction = MetricsExp.internalData.simpleCounterMetricMap[NamesConfigMap].GetLabel
	servicesLabelFunction = MetricsExp.internalData.simpleCounterMetricMap[NamesServices].GetLabel
	roleBindingLabelFunction = MetricsExp.internalData.simpleCounterMetricMap[NamesRoleBindings].GetLabel
	VMOUpdateLabelFunction = MetricsExp.internalData.simpleCounterMetricMap[NamesVMOUpdate].GetLabel
}

func InitRegisterStart() {
	RequiredInitialization()
	RegisterMetrics()
	StartMetricsServer()
}

func (md *metricsDelegate) TestInitialization() {
	RequiredInitialization()
}

func RegisterMetrics() {
	MetricsExp.internalMetricsDelegate.InitializeAllMetricsArray()  //populate allMetrics array with all map values
	go MetricsExp.internalMetricsDelegate.RegisterMetricsHandlers() //begin the retry process
}

func StartMetricsServer() {
	go wait.Until(func() {
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(":9100", nil)
		if err != nil {
			zap.S().Errorf("Failed to start metrics server for VMI: %v", err)
		}
	}, time.Second*3, wait.NeverStop)
}

func (md *metricsDelegate) InitializeAllMetricsArray() {
	//loop through all metrics declarations in metric maps
	for _, value := range MetricsExp.internalData.functionMetricsMap {
		MetricsExp.internalConfig.allMetrics = append(MetricsExp.internalConfig.allMetrics, value.callsTotal.metric, value.durationSeconds.metric, value.errorTotal.metric, value.lastCallTimestamp.metric, value.durationSeconds.metric)
	}
	for _, value := range MetricsExp.internalData.simpleCounterMetricMap {
		MetricsExp.internalConfig.allMetrics = append(MetricsExp.internalConfig.allMetrics, value.metric)
	}
	for _, value := range MetricsExp.internalData.durationMetricMap {
		MetricsExp.internalConfig.allMetrics = append(MetricsExp.internalConfig.allMetrics, value.metric)
	}
	for _, value := range MetricsExp.internalData.timestampMetricMap {
		MetricsExp.internalConfig.allMetrics = append(MetricsExp.internalConfig.allMetrics, value.metric)
	}
	for _, value := range MetricsExp.internalData.errorMetricMap {
		MetricsExp.internalConfig.allMetrics = append(MetricsExp.internalConfig.allMetrics, value.metric)
	}
}

func (md *metricsDelegate) RegisterMetricsHandlers() {
	md.initializeFailedMetricsArray() //Get list of metrics to register initially
	//loop until there is no error in registering
	for err := md.registerMetricsHandlersHelper(); err != nil; err = md.registerMetricsHandlersHelper() {
		zap.S().Errorf("Failed to register metrics for VMI %v \n", err)
		time.Sleep(time.Second)
	}
}

func (md *metricsDelegate) GetAllMetricsArray() *[]prometheus.Collector {
	return &MetricsExp.internalConfig.allMetrics
}

func (md *metricsDelegate) GetFailedMetricsMap() map[prometheus.Collector]int {
	return MetricsExp.internalConfig.failedMetrics
}

//nolint
func GetFunctionMetrics(name metricName) *functionMetrics {
	returnVal, found := MetricsExp.internalData.functionMetricsMap[name]
	if !found {
		zap.S().Errorf("%v is not a valid function metric, it is not in the functionMetrics map", name)
	}
	return returnVal
}

func (md *metricsDelegate) GetFunctionTimestampMetric(name metricName) *prometheus.GaugeVec {
	return MetricsExp.internalData.functionMetricsMap[name].lastCallTimestamp.metric
}

func (md *metricsDelegate) GetFunctionDurationMetric(name metricName) prometheus.Summary {
	return MetricsExp.internalData.functionMetricsMap[name].durationSeconds.metric
}

func (md *metricsDelegate) GetFunctionErrorMetric(name metricName) *prometheus.CounterVec {
	return MetricsExp.internalData.functionMetricsMap[name].errorTotal.metric
}

func (md *metricsDelegate) GetFunctionCounterMetric(name metricName) prometheus.Counter {
	return MetricsExp.internalData.functionMetricsMap[name].callsTotal.metric
}

//nolint
func GetSimpleCounterMetrics(name metricName) *simpleCounterMetric {
	return MetricsExp.internalData.simpleCounterMetricMap[name]
}

//nolint
func GetSimpleGaugeMetrics(name metricName) *simpleGaugeMetric {
	return MetricsExp.internalData.simpleGaugeMetricMap[name]
}

//nolint
func GetErrorMetrics(name metricName) *errorMetric {
	return MetricsExp.internalData.errorMetricMap[name]
}

//nolint
func GetDurationMetrics(name metricName) *durationMetric {
	return MetricsExp.internalData.durationMetricMap[name]
}

//nolint
func GetTimestampMetrics(name metricName) *timestampMetric {
	return MetricsExp.internalData.timestampMetricMap[name]
}
