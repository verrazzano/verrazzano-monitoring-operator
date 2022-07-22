// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package metricsexporter

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
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
	NamesServicesCreated         metricName = "servicesCreated"
	NamesServices                metricName = "services"
	NamesRoleBindings            metricName = "roleBindings"
	NamesIngress                 metricName = "ingress"
	NamesIngressDeleted          metricName = "ingressDeleted"
	NamesVMOUpdate               metricName = "vmoupdate"
	NamesQueue                   metricName = "queue"
)

type metricsExporter struct {
	internalMetricsDelegate metricsDelegate
	internalConfig          configuration
	internalData            data
}

type configuration struct {
	allMetrics    []prometheus.Collector       //thisMetric array will be automatically populated with all the metrics from each map. Metrics not included in a map can be added to thisMetric array for registration.
	failedMetrics map[prometheus.Collector]int //thisMetric map will be automatically populated with all metrics which were not registered correctly. Metrics in thisMetric map will be retried periodically.
	registry      prometheus.Registerer
}

type data struct {
	functionMetricsMap     map[metricName]*functionMetrics
	simpleCounterMetricMap map[metricName]*simpleCounterMetric
	simpleGaugeMetricMap   map[metricName]*simpleGaugeMetric
	durationMetricMap      map[metricName]*durationMetric
	timestampMetricMap     map[metricName]*timestampMetric
	errorMetricMap         map[metricName]*errorMetric
}

type metricsDelegate struct {
}

//Class of metrics to automatically capture 4 types of metrics for a given function
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
		NamesReconcile: {
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

		NamesDeployment: {
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

		NamesIngress: {
			durationSeconds: durationMetric{
				metric: prometheus.NewSummary(prometheus.SummaryOpts{Name: "vmo_ingress_duration_seconds", Help: "Tracks the duration of the ingress function in seconds"}),
			},
			callsTotal: simpleCounterMetric{
				metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_ingress_total", Help: "Tracks how many times the ingress function is called. This metric corresponds to the number of ingress requests performed by the VMO"}),
			},
			lastCallTimestamp: timestampMetric{
				metric: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmo_ingress_last_timestamp_seconds", Help: "The timestamp of the last time the ingress function completed"}, []string{"reconcile_index"}),
			},
			errorTotal: errorMetric{
				metric: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_ingress_error_total", Help: "Tracks how many times the syncHandlerStandardMode function encounters an error"}, []string{"reconcile_index"}),
			},
			index:         int64(0),
			labelFunction: &DefaultLabelFunction,
		},
	}
}

//nolint
func initSimpleCounterMetricMap() map[metricName]*simpleCounterMetric {
	return map[metricName]*simpleCounterMetric{
		NamesDeploymentUpdateCounter: {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_deployment_update_total", Help: "Tracks how many times a deployment update is attempted"}),
		},
		NamesDeploymentDeleteCounter: {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_deployment_delete_total", Help: "Tracks how many times the delete functionality is invoked"}),
		},
		NamesIngressDeleted: {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_ingress_delete_total", Help: "Tracks how many ingresses are deleted"}),
		},
		NamesConfigMap: {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_configmap_total", Help: "Tracks how many times the configMap functionality is invoked"}),
		},
		NamesServices: {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_services_total", Help: "Tracks how many times the services functionality is invoked"}),
		},
		NamesServicesCreated: {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_services_created_total", Help: "Tracks how many services are created"}),
		},
		NamesRoleBindings: {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_rolebindings_total", Help: "Tracks how many times the rolebindings functionality is invoked"}),
		},
		NamesVMOUpdate: {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_updates_total", Help: "Tracks how many times the update functionality is invoked"}),
		},
	}
}

//nolint
func initSimpleGaugeMetricMap() map[metricName]*simpleGaugeMetric {
	return map[metricName]*simpleGaugeMetric{
		NamesQueue: {
			metric: prometheus.NewGauge(prometheus.GaugeOpts{Name: "vmo_work_queue_size", Help: "Tracks the size of the VMO work queue"}),
		},
	}
}

//nolint
func initDurationMetricMap() map[metricName]*durationMetric {
	return map[metricName]*durationMetric{}
}

//nolint
func initTimestampMetricMap() map[metricName]*timestampMetric {
	return map[metricName]*timestampMetric{
		NamesConfigMap: {
			metric:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmo_configmap_last_succesful_timestamp", Help: "The timestamp of the last time the configMap function completed successfully"}, []string{"reconcile_index"}),
			labelFunction: &configMapLabelFunction,
		},
		NamesServices: {
			metric:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmo_services_last_succesful_timestamp", Help: "The timestamp of the last time the createService function completed successfully"}, []string{"reconcile_index"}),
			labelFunction: &servicesLabelFunction,
		},
		NamesRoleBindings: {
			metric:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmo_rolebindings_last_succesful_timestamp", Help: "The timestamp of the last time the roleBindings function completed successfully"}, []string{"reconcile_index"}),
			labelFunction: &roleBindingLabelFunction,
		},
		NamesVMOUpdate: {
			metric:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmo_update_last_succesful_timestamp", Help: "The timestamp of the last time the vmo update completed successfully"}, []string{"reconcile_index"}),
			labelFunction: &VMOUpdateLabelFunction,
		},
	}
}

//nolint
func initErrorMetricMap() map[metricName]*errorMetric {
	return map[metricName]*errorMetric{
		NamesDeploymentUpdateError: {
			metric:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_deployment_update_error_total", Help: "Tracks how many times a deployment update fails"}, []string{"reconcile_index"}),
			labelFunction: &deploymentLabelFunction,
		},
		NamesDeploymentDeleteError: {
			metric:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_deployment_delete_error_counter", Help: "Tracks how many times the delete functionality failed"}, []string{"reconcile_index"}),
			labelFunction: &deploymentLabelFunction,
		},
	}
}

var (
	MetricsExp               = metricsExporter{}
	DefaultLabelFunction     func(index int64) string
	deploymentLabelFunction  func() string
	configMapLabelFunction   func() string
	servicesLabelFunction    func() string
	roleBindingLabelFunction func() string
	VMOUpdateLabelFunction   func() string
	emptyLabelFunction       = func() string { return "default" }
	TestDelegate             = metricsDelegate{}
)

func (md *metricsDelegate) initializeFailedMetricsArray() {
	//the failed metrics array will initially contain all metrics so they may be registered
	for i, metric := range MetricsExp.internalConfig.allMetrics {
		MetricsExp.internalConfig.failedMetrics[metric] = i
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

func numToString(num int64) string {
	return fmt.Sprintf("%d", num)
}
