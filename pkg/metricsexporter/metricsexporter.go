// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package metricsExporter

import (
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

type metricName string

const (
	NamesReconcile               metricName = "reconcile"
	NamesDeployment              metricName = "deployment"
	NamesDeploymentUpdateError   metricName = "deploymentUpdateErrorCounter"
	NamesDeploymentDeleteCounter metricName = "deploymentDeleteCounter"
	NamesDeploymentDeleteError   metricName = "deploymentDeleteErrorCounter"
	NamesDeploymentUpdateCounter metricName = "deploymentUpdateCounter"
	NamesConfigMap               metricName = "configMap"
)

//nolint
type metricsExporter struct {
	internalMetricsDelegate metricsDelegate
	internalConfig          configuration
	internalData            data
}

//nolint
type configuration struct {
	allMetrics    []prometheus.Collector       //thisMetric array will be automatically populated with all the metrics from each map. Metrics not included in a map can be added to thisMetric array for registration.
	failedMetrics map[prometheus.Collector]int //thisMetric map will be automatically populated with all metrics which were not registered correctly. Metrics in thisMetric map will be retried periodically.
	registry      prometheus.Registerer
}

//nolint
type data struct {
	functionMetricsMap     map[metricName]*functionMetrics
	simpleCounterMetricMap map[metricName]*simpleCounterMetric
	simpleGaugeMetricMap   map[metricName]*simpleGaugeMetric
	durationMetricMap      map[metricName]*durationMetric
	timestampMetricMap     map[metricName]*timestampMetric
	errorMetricMap         map[metricName]*errorMetric
}

//nolint
type metricsDelegate struct {
}

//Class of metrics to automatically capture 4 types of metrics for a given function
//nolint
type functionMetrics struct {
	durationSeconds   durationMetric
	callsTotal        simpleCounterMetric
	lastCallTimestamp timestampMetric
	errorTotal        errorMetric
	labelFunction     *func(int64) string //The function to create the label values for the error and timestamp metrics. A default is provided as &DefaultLabelFunction
	index             int64
}

//Method to call at the start of the tracked function. Starts the duration timer and increments the total count
func (f *functionMetrics) LogStart() {
	f.callsTotal.metric.Inc()
	f.index = f.index + 1
	f.durationSeconds.TimerStart()
}

//Method to defer to the end of the tracked function. Stops the duration timer, sets the lastCallTimestamp. Pass in an argument of true to set an error for the current function call.
func (f *functionMetrics) LogEnd(errorObserved bool) {
	label := (*f.labelFunction)(f.index)
	f.durationSeconds.TimerStop()
	f.lastCallTimestamp.SetLastTimeWithLabel(label)
	if errorObserved {
		f.errorTotal.IncWithLabel(label)
	}
}

func (f *functionMetrics) IncError() {
	f.errorTotal.IncWithLabel(f.GetLabel())
}

//Invokes the supplied labelFunction to return the string which would be used as a label. The label can be dynamic and may change depending on the labelFunctions behavior (i.e. a timestamp string)
func (f *functionMetrics) GetLabel() string {
	return (*f.labelFunction)(f.index)
}

//Type to count events such as the number fo function calls.
//nolint
type simpleCounterMetric struct {
	metric prometheus.Counter
	index  int64
}

func (c *simpleCounterMetric) Inc() {
	c.index = c.index + 1
	c.metric.Inc()
}

func (c *simpleCounterMetric) Add(num float64) {
	c.index = c.index + int64(num)
	c.metric.Add(num)
}

func (c *simpleCounterMetric) GetLabel() string {
	return numToString(c.index)
}

//nolint
type simpleGaugeMetric struct {
	metric prometheus.Gauge
}

func (g *simpleGaugeMetric) Set(num float64) {
	g.metric.Set(num)
}

func (g *simpleGaugeMetric) SetToCurrentTime() {
	g.metric.SetToCurrentTime()
}

func (g *simpleGaugeMetric) Add(num float64) {
	g.metric.Add(num)
}

//Type to track length of a function call. Method to start and stop the duration timer are available.
//nolint
type durationMetric struct {
	metric prometheus.Summary
	timer  *prometheus.Timer
}

//Creates a new timer, and starts the timer
func (d *durationMetric) TimerStart() {
	d.timer = prometheus.NewTimer(d.metric)
}

//stops the timer and record the duration since the last call to TimerStart
func (d *durationMetric) TimerStop() {
	d.timer.ObserveDuration()
}

//Type to track the last timestamp of a function call. Includes a method to set the last timestamp
//nolint
type timestampMetric struct {
	metric        *prometheus.GaugeVec
	labelFunction *func() string
}

//Adds a timestamp as the current time. The label must be supplied as an argument
func (t *timestampMetric) SetLastTime() {
	t.SetLastTimeWithLabel((*t.labelFunction)())
}

//Adds a timestamp as the current time. The label must be supplied as an argument
func (t *timestampMetric) SetLastTimeWithLabel(indexString string) {
	lastTimeMetric, err := t.metric.GetMetricWithLabelValues(indexString)
	if err != nil {
		zap.S().Errorf("Failed to log the last reconcile time metric label %s: %v", indexString, err)
	} else {
		lastTimeMetric.SetToCurrentTime()
	}
}

//Type to track the occurrence of an error. Includes a metod to add an error count
//nolint
type errorMetric struct {
	metric        *prometheus.CounterVec
	labelFunction *func() string
}

func (e *errorMetric) Inc() {
	e.IncWithLabel((*e.labelFunction)())
}

//Adds an error count. The label must be supplied as an argument
func (e *errorMetric) IncWithLabel(label string) {
	errorMetric, err := e.metric.GetMetricWithLabelValues(label)
	if err != nil {
		zap.S().Errorf("Failed to get metric label %s: %v", label, err)
	} else {
		errorMetric.Inc()
	}
}

func initConfiguration() configuration {
	return configuration{
		allMetrics:    []prometheus.Collector{},
		failedMetrics: map[prometheus.Collector]int{},
		registry:      prometheus.DefaultRegisterer,
	}
}

func initFunctionMetricsMap() map[metricName]*functionMetrics {
	return map[metricName]*functionMetrics{
		"reconcile": {
			durationSeconds: durationMetric{
				metric: prometheus.NewSummary(prometheus.SummaryOpts{Name: "vmo_reconcile_duration_seconds", Help: "Tracks the duration of the reconcile function in seconds"}),
			},
			callsTotal: simpleCounterMetric{
				metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_reconcile_total", Help: "Tracks how many times the syncHandlerStandardMode function is called. thisMetric corresponds to the number of reconciles performed by the VMO"}),
			},
			lastCallTimestamp: timestampMetric{
				metric: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmo_reconcile_last_timestamp_seconds", Help: "The timestamp of the last time the syncHandlerStandardMode function completed"}, []string{"reconcile_index"}),
			},
			errorTotal: errorMetric{
				metric: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_reconcile_error_total", Help: "Tracks how many times the syncHandlerStandardMode function encounters an error"}, []string{"reconcile_index"}),
			},
			index:         int64(0),
			labelFunction: &DefaultLabelFunction,
		},

		"deployment": {
			durationSeconds: durationMetric{
				metric: prometheus.NewSummary(prometheus.SummaryOpts{Name: "vmo_deployment_duration_seconds", Help: "The duration of the last call to the deployment function"}),
			},
			callsTotal: simpleCounterMetric{
				metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_deployment_total", Help: "Tracks how many times the deployment function is called"}),
			},
			lastCallTimestamp: timestampMetric{
				metric: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmo_deployment_last_timestamp_seconds", Help: "The timestamp of the last time the deployment function completed"}, []string{"reconcile_index"}),
			},
			errorTotal: errorMetric{
				metric: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_deployment_error_total", Help: "Tracks how many times the deployment failed"}, []string{"reconcile_index"}),
			},
			index:         int64(0),
			labelFunction: &DefaultLabelFunction,
		},
	}
}

func initSimpleCounterMetricMap() map[metricName]*simpleCounterMetric {
	return map[metricName]*simpleCounterMetric{
		"deploymentUpdateCounter": {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_deployment_update_total", Help: "Tracks how many times a deployment update is attempted"}),
		},
		"deploymentDeleteCounter": {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_deployment_delete_counter", Help: "Tracks how many times the delete functionality is invoked"}),
		},
	}
}

func initSimpleGaugeMetricMap() map[metricName]*simpleGaugeMetric {
	return map[metricName]*simpleGaugeMetric{}
}

func initDurationMetricMap() map[metricName]*durationMetric {
	return map[metricName]*durationMetric{}
}

func initTimestampMetricMap() map[metricName]*timestampMetric {
	return map[metricName]*timestampMetric{
		"configMap": {
			metric:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmo_deployment_last_timestamp_seconds", Help: "The timestamp of the last time the deployment function completed"}, []string{"reconcile_index"}),
			labelFunction: &configMapLabelFunction,
		},
	}
}

func initErrorMetricMap() map[metricName]*errorMetric {
	return map[metricName]*errorMetric{
		"deploymentUpdateErrorCounter": {
			metric:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_deployment_update_error_total", Help: "Tracks how many times a deployment update fails"}, []string{"reconcile_index"}),
			labelFunction: &deploymentLabelFunction,
		},
		"deploymentDeleteErrorCounter": {
			metric:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_deployment_delete_error_counter", Help: "Tracks how many times the delete functionality failed"}, []string{"reconcile_index"}),
			labelFunction: &deploymentLabelFunction,
		},
	}
}

var (
	MetricsExp              = metricsExporter{}
	DefaultLabelFunction    func(index int64) string
	deploymentLabelFunction func() string
	configMapLabelFunction  func() string
	TestDelegate            = metricsDelegate{}
)

func InitRegisterStart() {
	RequiredInitialization()
	RegisterMetrics()
	StartMetricsServer()
}

func (md *metricsDelegate) TestInitialization() {
	RequiredInitialization()
}

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
	configMapLabelFunction = func() string {
		return numToString(MetricsExp.internalData.simpleCounterMetricMap[NamesConfigMap].index)
	}
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

func GetFunctionMetrics(name metricName) *functionMetrics {
	return MetricsExp.internalData.functionMetricsMap[name]
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

func GetSimpleCounterMetrics(name metricName) *simpleCounterMetric {
	return MetricsExp.internalData.simpleCounterMetricMap[name]
}
func GetSimpleGaugeMetrics(name metricName) *simpleGaugeMetric {
	return MetricsExp.internalData.simpleGaugeMetricMap[name]
}
func GetErrorMetrics(name metricName) *errorMetric {
	return MetricsExp.internalData.errorMetricMap[name]
}
func GetDurationMetrics(name metricName) *durationMetric {
	return MetricsExp.internalData.durationMetricMap[name]
}
func GetTimestampMetrics(name metricName) *timestampMetric {
	return MetricsExp.internalData.timestampMetricMap[name]
}

func (md *metricsDelegate) initializeFailedMetricsArray() {
	//the failed metrics array will initially contain all metrics so they may be registered
	for i, metric := range MetricsExp.internalConfig.allMetrics {
		MetricsExp.internalConfig.failedMetrics[metric] = i
	}
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

func (md *metricsDelegate) registerMetricsHandlersHelper() error {
	var errorObserved error
	for metric := range MetricsExp.internalConfig.failedMetrics {
		err := MetricsExp.internalConfig.registry.Register(metric)
		if err != nil {
			if errorObserved != nil {
				errorObserved = errors.Wrap(errorObserved, err.Error())
			} else {
				errorObserved = err
			}
		} else {
			//if a metric is registered, delete it from the failed metrics map so that it is not retried
			delete(MetricsExp.internalConfig.failedMetrics, metric)
		}
	}
	return errorObserved
}

func (md *metricsDelegate) GetAllMetricsArray() *[]prometheus.Collector {
	return &MetricsExp.internalConfig.allMetrics
}

func (md *metricsDelegate) GetFailedMetricsMap() map[prometheus.Collector]int {
	return MetricsExp.internalConfig.failedMetrics
}

func numToString(num int64) string {
	return fmt.Sprintf("%d", num)
}
