// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

//is retry functionality good to have? Should I look to support progmatically creating metrics? How about flags, is that a possible feature in the future? Im thinking about using some sort of struct or member functions to make all the metrics generic, how does that sound?

type FunctionMetrics struct {
	durationSeconds   DurationMetric
	callsTotal        SimpleCounterMetric
	lastCallTimestamp TimestampMetric
	errorTotal        ErrorMetric
	index             int64
	labelFunction     *func(int64) string
}

func (this *FunctionMetrics) LogStart() {
	this.callsTotal.metric.Inc()
	this.index = this.index + 1
	this.durationSeconds.TimerStart()
}

func (this *FunctionMetrics) LogEnd(errorObserved bool) {
	label := (*this.labelFunction)(this.index)
	this.durationSeconds.TimerStop()
	this.lastCallTimestamp.setLastTime(label)
	if errorObserved {
		this.errorTotal.metricVecErrorIncrement(label)
	}
}

func (this *FunctionMetrics) GetLabel() string {
	return (*this.labelFunction)(this.index)
}

type SimpleCounterMetric struct {
	metric prometheus.Counter
}

type DurationMetric struct {
	metric prometheus.Summary
	timer  *prometheus.Timer
}

func (this *DurationMetric) TimerStart() {
	this.timer = prometheus.NewTimer(this.metric)
}

func (this *DurationMetric) TimerStop() {
	this.timer.ObserveDuration()
}

type TimestampMetric struct {
	metric *prometheus.GaugeVec
}

func (this *TimestampMetric) setLastTime(indexString string) {
	lastTimeMetric, err := this.metric.GetMetricWithLabelValues(indexString)
	if err != nil {
		zap.S().Errorf("Failed to log the last reconcile time metric label %s: %v", indexString, err)
	} else {
		lastTimeMetric.SetToCurrentTime()
	}
}

type ErrorMetric struct {
	metric *prometheus.CounterVec
}

func (this *ErrorMetric) metricVecErrorIncrement(label string) {
	errorMetric, err := this.metric.GetMetricWithLabelValues(label)
	if err != nil {
		zap.S().Errorf("Failed to get metric label %s: %v", label, err)
	} else {
		errorMetric.Inc()
	}
}

var (
	FunctionMetricsMap = map[string]*FunctionMetrics{
		//syncHandler/Reconcile metric
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
	//VMO deployments metrics

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
	allMetrics           []prometheus.Collector
	failedMetrics        = map[prometheus.Collector]int{}
	registry             = prometheus.DefaultRegisterer
)

func StartMetricsServer() {

	initializeAllMetricsArray()
	go registerMetricsHandlers()

	go wait.Until(func() {
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(":9100", nil)
		if err != nil {
			zap.S().Errorf("Failed to start metrics server for VMI: %v", err)
		}
	}, time.Second*3, wait.NeverStop)

}

func initializeFailedMetricsArray() {
	for i, metric := range allMetrics {
		failedMetrics[metric] = i
	}
}

func initializeAllMetricsArray() {
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
	initializeFailedMetricsArray()
	for err := registerMetricsHandlersHelper(); err != nil; err = registerMetricsHandlersHelper() {
		zap.S().Errorf("Failed to register some metrics for VMI: %v", err)
		time.Sleep(time.Second)
	}
}

func registerMetricsHandlersHelper() error {
	var errorObserved error
	for metric, i := range failedMetrics {
		err := registry.Register(metric)
		if err != nil {
			zap.S().Errorf("Failed to register metric index %v for VMI", i)
			errorObserved = err
		} else {
			delete(failedMetrics, metric)
		}
	}
	return errorObserved
}

//testutil.tfloat64

func numToString(num int64) string {
	return fmt.Sprintf("%d", num)
}
