// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

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

//Class of metrics to automatically capture 4 types of metrics for a given function
type FunctionMetrics struct {
	durationSeconds   DurationMetric
	callsTotal        SimpleCounterMetric
	lastCallTimestamp TimestampMetric
	errorTotal        ErrorMetric
	index             int64
	labelFunction     *func(int64) string //The function to create the label values for the error and timestamp metrics. A default is provided as &DefaultLabelFunction
}

//Method to call at the start of the tracked function. Starts the duration timer and increments the total count
func (this *FunctionMetrics) LogStart() {
	this.callsTotal.metric.Inc()
	this.index = this.index + 1
	this.durationSeconds.TimerStart()
}

//Method to defer to the end of the tracked function. Stops the duration timer, sets the lastCallTimestamp. Pass in an argument of true to set an error for the current function call.
func (this *FunctionMetrics) LogEnd(errorObserved bool) {
	label := (*this.labelFunction)(this.index)
	this.durationSeconds.TimerStop()
	this.lastCallTimestamp.setLastTime(label)
	if errorObserved {
		this.errorTotal.metricVecErrorIncrement(label)
	}
}

//Invokes the supplied labelFunction to return the string which would be used as a label. The label can be dynamic and may change depending on the labelFunctions behavior (i.e. a timestamp string)
func (this *FunctionMetrics) GetLabel() string {
	return (*this.labelFunction)(this.index)
}

//Type to count events such as the number fo function calls.
type SimpleCounterMetric struct {
	metric prometheus.Counter
}

//Type to track length of a function call. Method to start and stop the duration timer are available.
type DurationMetric struct {
	metric prometheus.Summary
	timer  *prometheus.Timer
}

//Creates a new timer, and starts the timer
func (this *DurationMetric) TimerStart() {
	this.timer = prometheus.NewTimer(this.metric)
}

//stops the timer and record the duration since the last call to TimerStart
func (this *DurationMetric) TimerStop() {
	this.timer.ObserveDuration()
}

//Type to track the last timestamp of a function call. Includes a method to set the last timestamp
type TimestampMetric struct {
	metric *prometheus.GaugeVec
}

//Adds a timestamp as the current time. The label must be supplied as an argument
func (this *TimestampMetric) setLastTime(indexString string) {
	lastTimeMetric, err := this.metric.GetMetricWithLabelValues(indexString)
	if err != nil {
		zap.S().Errorf("Failed to log the last reconcile time metric label %s: %v", indexString, err)
	} else {
		lastTimeMetric.SetToCurrentTime()
	}
}

//Type to track the occurrence of an error. Includes a metod to add an error count
type ErrorMetric struct {
	metric *prometheus.CounterVec
}

//Adds an error count. The label must be supplied as an argument
func (this *ErrorMetric) metricVecErrorIncrement(label string) {
	errorMetric, err := this.metric.GetMetricWithLabelValues(label)
	if err != nil {
		zap.S().Errorf("Failed to get metric label %s: %v", label, err)
	} else {
		errorMetric.Inc()
	}
}

var (
	//Metrics can be accessed through the metrics maps. All metrics declared in the metrics maps or allmetrics array are registered automatically.
	FunctionMetricsMap = map[string]*FunctionMetrics{
		"reconcile": {
			durationSeconds: DurationMetric{
				metric: prometheus.NewSummary(prometheus.SummaryOpts{Name: "vmo_reconcile_duration_seconds", Help: "Tracks the duration of the reconcile function in seconds"}),
			},
			callsTotal: SimpleCounterMetric{
				metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_reconcile_total", Help: "Tracks how many times the syncHandlerStandardMode function is called. This corresponds to the number of reconciles performed by the VMO"}),
			},
			lastCallTimestamp: TimestampMetric{
				metric: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmo_reconcile_last_timestamp_seconds", Help: "The timestamp of the last time the syncHandlerStandardMode function completed"}, []string{"reconcile_index"}),
			},
			errorTotal: ErrorMetric{
				metric: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_reconcile_error_total", Help: "Tracks how many times the syncHandlerStandardMode function encounters an error"}, []string{"reconcile_index"}),
			},
			index:         int64(0),
			labelFunction: &DefaultLabelFunction,
		},

		"deployment": {
			durationSeconds: DurationMetric{
				metric: prometheus.NewSummary(prometheus.SummaryOpts{Name: "vmo_deployment_duration_seconds", Help: "The duration of the last call to the deployment function"}),
			},
			callsTotal: SimpleCounterMetric{
				metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_deployment_total", Help: "Tracks how many times the deployment function is called"}),
			},
			lastCallTimestamp: TimestampMetric{
				metric: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "vmo_deployment_last_timestamp_seconds", Help: "The timestamp of the last time the deployment function completed"}, []string{"reconcile_index"}),
			},
			errorTotal: ErrorMetric{
				metric: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_deployment_error_total", Help: "Tracks how many times the deployment failed"}, []string{"reconcile_index"}),
			},
			index:         int64(0),
			labelFunction: &DefaultLabelFunction,
		},
	}

	SimpleCounterMetricMap = map[string]*SimpleCounterMetric{
		"deploymentUpdateCounter": {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_deployment_update_total", Help: "Tracks how many times a deployment update is attempted"}),
		},
		"deploymentDeleteCounter": {
			metric: prometheus.NewCounter(prometheus.CounterOpts{Name: "vmo_deployment_delete_counter", Help: "Tracks how many times the delete functionality is invoked"}),
		},
	}

	DurationMetricMap = map[string]*DurationMetric{}

	TimestampMetricMap = map[string]*TimestampMetric{}

	ErrorMetricMap = map[string]*ErrorMetric{
		"deploymentUpdateErrorCounter": {
			metric: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_deployment_update_error_total", Help: "Tracks how many times a deployment update fails"}, []string{"reconcile_index"}),
		},
		"deploymentDeleteErrorCounter": {
			metric: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "vmo_deployment_delete_error_counter", Help: "Tracks how many times the delete functionality failed"}, []string{"reconcile_index"}),
		},
	}

	DefaultLabelFunction = func(index int64) string { return numToString(index) }
	//This array will be automatically populated with all the metrics from each map. Metrics not included in a map can be added to this array for registration.
	allMetrics []prometheus.Collector
	//This map will be automatically populated with all metrics which were not registered correctly. Metrics in this map will be retried periodically.
	failedMetrics = map[prometheus.Collector]int{}
	registry      = prometheus.DefaultRegisterer
)

func StartMetricsServer() {

	initializeAllMetricsArray()  //populate allMetrics array with all map values
	go registerMetricsHandlers() //begin the retry process

	go wait.Until(func() {
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(":9100", nil)
		if err != nil {
			zap.S().Errorf("Failed to start metrics server for VMI: %v", err)
		}
	}, time.Second*3, wait.NeverStop)

}

func initializeFailedMetricsArray() {
	//the failed metrics array will initially contain all metrics so they may be registered
	for i, metric := range allMetrics {
		failedMetrics[metric] = i
	}
}

func initializeAllMetricsArray() {
	//loop through all metrics declarations in metric maps
	for _, value := range FunctionMetricsMap {
		allMetrics = append(allMetrics, value.callsTotal.metric, value.durationSeconds.metric, value.errorTotal.metric, value.lastCallTimestamp.metric, value.durationSeconds.metric)
	}
	for _, value := range SimpleCounterMetricMap {
		allMetrics = append(allMetrics, value.metric)
	}
	for _, value := range DurationMetricMap {
		allMetrics = append(allMetrics, value.metric)
	}
	for _, value := range TimestampMetricMap {
		allMetrics = append(allMetrics, value.metric)
	}
	for _, value := range ErrorMetricMap {
		allMetrics = append(allMetrics, value.metric)
	}
}

func registerMetricsHandlers() {
	initializeFailedMetricsArray() //Get list of metrics to register initially
	//loop until there is no error in registering
	for err := registerMetricsHandlersHelper(); err != nil; err = registerMetricsHandlersHelper() {
		zap.S().Errorf("Failed to register metrics for VMI %v \n", err)
		time.Sleep(time.Second)
	}
}

func registerMetricsHandlersHelper() error {
	var errorObserved error
	for metric := range failedMetrics {
		err := registry.Register(metric)
		if err != nil {
			if errorObserved != nil {
				errorObserved = errors.Wrap(errorObserved, err.Error())
			} else {
				errorObserved = err
			}
		} else {
			//if a metric is registered, delete it from the failed metrics map so that it is not retried
			delete(failedMetrics, metric)
		}
	}
	return errorObserved
}

func numToString(num int64) string {
	return fmt.Sprintf("%d", num)
}
