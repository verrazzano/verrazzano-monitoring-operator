// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package metricsexporter

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

var (
	//syncHandler/Reconcile metric
	reconcileIndex        uint64 = 0
	reconcileTimer        *prometheus.Timer
	reconcileCounter      = prometheus.NewCounter(prometheus.CounterOpts{Name: "reconcileCounter", Help: "Tracks how many times the syncHandlerStandardMode function is called. This corresponds to the number of reconciles performed by the VMO"})
	reconcileLastTime     = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "reconcileLastTime", Help: "The timestamp of the last time the syncHandlerStandardMode function completed"}, []string{"reconcile_index"})
	reconcileErrorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "reconcileErrorCounter", Help: "Tracks how many times the syncHandlerStandardMode function encounters an error"}, []string{"reconcile_index"})
	reconcileDuration     = prometheus.NewSummary(prometheus.SummaryOpts{Name: "reconcileLastTime", Help: "The timestamp of the last time the syncHandlerStandardMode function completed"})

	//VMO deployments metrics
	deploymentIndex                     uint64 = 0
	deploymentTimer                     *prometheus.Timer
	deploymentCounter                   = prometheus.NewCounter(prometheus.CounterOpts{Name: "deploymentCounter", Help: "Tracks how many times the syncHandlerStandardMode function is called. This corresponds to the number of reconciles performed by the VMO"})
	deploymentLastTime                  = prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "deploymentLastTime", Help: "The timestamp of the last time the syncHandlerStandardMode function completed"}, []string{"reconcile_index"})
	deploymentErrorCounter              = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deploymentErrorCounter", Help: "Tracks how many times the syncHandlerStandardMode function encounters an error"}, []string{"reconcile_index"})
	deploymentDuration                  = prometheus.NewSummary(prometheus.SummaryOpts{Name: "deploymentLastTime", Help: "The timestamp of the last time the syncHandlerStandardMode function completed"})
	deploymentUpdateCounter             = prometheus.NewCounter(prometheus.CounterOpts{Name: "deploymentCounter", Help: "Tracks how many times the syncHandlerStandardMode function is called. This corresponds to the number of reconciles performed by the VMO"})
	deploymentUpdateErrorCounter        = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deploymentErrorCounter", Help: "Tracks how many times the syncHandlerStandardMode function encounters an error"}, []string{"reconcile_index"})
	deploymentRollingUpdateCounter      = prometheus.NewCounter(prometheus.CounterOpts{Name: "deploymentCounter", Help: "Tracks how many times the syncHandlerStandardMode function is called. This corresponds to the number of reconciles performed by the VMO"})
	deploymentRollingUpdateErrorCounter = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deploymentErrorCounter", Help: "Tracks how many times the syncHandlerStandardMode function encounters an error"}, []string{"reconcile_index"})
	deploymentDeleteCounter             = prometheus.NewCounter(prometheus.CounterOpts{Name: "deploymentCounter", Help: "Tracks how many times the syncHandlerStandardMode function is called. This corresponds to the number of reconciles performed by the VMO"})
	deploymentDeleteErrorCounter        = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "deploymentErrorCounter", Help: "Tracks how many times the syncHandlerStandardMode function encounters an error"}, []string{"reconcile_index"})

	allMetrics    = []prometheus.Collector{reconcileCounter, reconcileLastTime, reconcileErrorCounter, reconcileDuration}
	failedMetrics = map[prometheus.Collector]int{}
	registry      = prometheus.DefaultRegisterer
)

func StartMetricsServer() {

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

func registerMetricsHandlers() {
	initializeFailedMetricsArray()
	for err := registerMetricsHandlersHelper(); err != nil; err = registerMetricsHandlersHelper() {
		zap.S().Errorf("Failed to register some metrics for VMI: %v", err)
		time.Sleep(time.Second)
	}
}

func registerMetricsHandlersHelper() error {
	var errorObserved error = nil
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

func metricCounterVecErrorIncrement(metricVec *prometheus.CounterVec, label string) {
	errorMetric, err := reconcileErrorCounter.GetMetricWithLabelValues(label)
	if err != nil {
		zap.S().Errorf("Failed to get metric label %s: %v", label, err)
	} else {
		errorMetric.Inc()
	}
}

func metricGaugeVecSetLastTime(metricVec *prometheus.GaugeVec, label string) {
	lastTimeMetric, err := metricVec.GetMetricWithLabelValues(label)
	if err != nil {
		zap.S().Errorf("Failed to log the last reconcile time metric label %s: %v", label, err)
	} else {
		lastTimeMetric.SetToCurrentTime()
	}
}

func numToString(num uint64) string {
	return fmt.Sprintf("%d", num)
}

/*

Begin SyncHandler/Reconcile Metrics

*/
func LogSyncHandlerStart() {
	ReconcileCountIncrement()
	ReconcileTimerStart()
}

func LogSyncHandlerEnd() {
	ReconcileTimerEnd()
	ReconcileLastTimeSet()
}

func ReconcileErrorIncrement() {
	indexString := numToString(reconcileIndex)
	metricCounterVecErrorIncrement(reconcileErrorCounter, indexString)
}

func ReconcileCountIncrement() {
	reconcileIndex++
	reconcileCounter.Inc()
}

func ReconcileLastTimeSet() {
	indexString := numToString(reconcileIndex)
	metricGaugeVecSetLastTime(reconcileLastTime, indexString)
}

func ReconcileTimerStart() {
	reconcileTimer = prometheus.NewTimer(reconcileDuration)
}

func ReconcileTimerEnd() {
	reconcileTimer.ObserveDuration()
}

/*

Begin Deployment Metrics

*/

func LogDeploymentStart() {
	DeploymentCountIncrement()
	DeploymentTimerStart()
}

func LogDeploymentEnd() {
	DeploymentTimerEnd()
	DeploymentLastTimeSet()
}

func DeploymentErrorIncrement() {
	indexString := numToString(deploymentIndex)
	metricCounterVecErrorIncrement(deploymentErrorCounter, indexString)
}

func DeploymentUpdateErrorIncrement() {
	indexString := numToString(deploymentIndex)
	metricCounterVecErrorIncrement(deploymentUpdateErrorCounter, indexString)
}

func DeploymentRollingUpdateErrorIncrement() {
	indexString := numToString(deploymentIndex)
	metricCounterVecErrorIncrement(deploymentRollingUpdateErrorCounter, indexString)
}

func DeploymentDeleteErrorIncrement() {
	indexString := numToString(deploymentIndex)
	metricCounterVecErrorIncrement(deploymentDeleteErrorCounter, indexString)
}

func DeploymentCountIncrement() {
	deploymentIndex++
	deploymentCounter.Inc()
}

//counts number of updates attempted, not just function calls
//in update, rolling update, update all, ...
func DeploymentUpdateCountIncrement() {
	deploymentUpdateCounter.Inc()
}

func DeploymentRollingUpdateCountIncrement() {
	deploymentRollingUpdateCounter.Inc()
}

func DeploymentDeleteCountIncrement() {
	deploymentDeleteCounter.Inc()
}

func DeploymentLastTimeSet() {
	indexString := numToString(deploymentIndex)
	metricGaugeVecSetLastTime(deploymentLastTime, indexString)
}

func DeploymentTimerStart() {
	deploymentTimer = prometheus.NewTimer(deploymentDuration)
}

func DeploymentTimerEnd() {
	deploymentTimer.ObserveDuration()
}
