// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package metricsexporter

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/wait"
)

// RequiredInitialization populates the metricsexporter, this function must be called before any other non-init exporter function is called
func RequiredInitialization() {
	MetricsExp = metricsExporter{
		internalMetricsDelegate: metricsDelegate{},
		internalConfig:          initConfiguration(),
		internalData: data{
			functionMetricsMap:     initFunctionMetricsMap(),
			simpleCounterMetricMap: initCounterMetricMap(),
			simpleGaugeMetricMap:   initGaugeMetricMap(),
			durationMetricMap:      initDurationMetricMap(),
			timestampMetricMap:     initTimestampMetricMap(),
			errorMetricMap:         initErrorMetricMap(),
		},
	}

	DefaultLabelFunction = func(index int64) string {
		return strconv.FormatInt(index, 10)
	}
	deploymentLabelFunction = MetricsExp.internalData.functionMetricsMap[NamesDeployment].GetLabel
	configMapLabelFunction = MetricsExp.internalData.simpleCounterMetricMap[NamesConfigMap].GetLabel
	servicesLabelFunction = MetricsExp.internalData.simpleCounterMetricMap[NamesServices].GetLabel
	roleBindingLabelFunction = MetricsExp.internalData.simpleCounterMetricMap[NamesRoleBindings].GetLabel
	VMOUpdateLabelFunction = MetricsExp.internalData.simpleCounterMetricMap[NamesVMOUpdate].GetLabel
}

// InitRegisterStart call this function in order to completely initialize and start the metrics exporter. Populates, registers, and starts the metrics server.
func InitRegisterStart() {
	RequiredInitialization()
	RegisterMetrics()
	StartMetricsServer()
}

// RegisterMetrics begins the registration process, Required Initialization must be called first. This function does not start the metrics server
func RegisterMetrics() {
	MetricsExp.internalMetricsDelegate.InitializeAllMetricsArray()  // populate allMetrics array with all map values
	go MetricsExp.internalMetricsDelegate.RegisterMetricsHandlers() // begin the retry process
}

func StartMetricsServer() {
	go wait.Until(func() {
		http.Handle("/metrics", promhttp.Handler())
		err := http.ListenAndServe(":9100", nil)
		if err != nil {
			zap.S().Errorf("Failed to start metrics server for VMO: %v", err)
		}
	}, time.Second*3, wait.NeverStop)
}

// InitializeAllMetricsArray internal function used to add all metrics from the metrics maps to the allMetrics array
func (md *metricsDelegate) InitializeAllMetricsArray() {
	// loop through all metrics declarations in metric maps
	for _, value := range MetricsExp.internalData.functionMetricsMap {
		MetricsExp.internalConfig.allMetrics = append(MetricsExp.internalConfig.allMetrics, value.callsTotal.metric, value.durationMetric.metric, value.errorTotal.metric, value.lastCallTimestamp.metric, value.durationMetric.metric)
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

// RegisterMetricsHandlers loops through the failedMetrics map until all metrics are registered successfully
func (md *metricsDelegate) RegisterMetricsHandlers() {
	md.initializeFailedMetricsArray() // Get list of metrics to register initially
	// loop until there is no error in registering
	for err := md.registerMetricsHandlersHelper(); err != nil; err = md.registerMetricsHandlersHelper() {
		zap.S().Errorf("Failed to register metrics for VMO %v \n", err)
		time.Sleep(time.Second)
	}
}

func (md *metricsDelegate) GetAllMetricsArray() *[]prometheus.Collector {
	return &MetricsExp.internalConfig.allMetrics
}

func (md *metricsDelegate) GetFailedMetricsMap() map[prometheus.Collector]int {
	return MetricsExp.internalConfig.failedMetrics
}

func (md *metricsDelegate) GetCounterMetric(name metricName) prometheus.Counter {
	return MetricsExp.internalData.simpleCounterMetricMap[name].metric
}

func (md *metricsDelegate) GetGaugeMetrics(name metricName) prometheus.Gauge {
	return MetricsExp.internalData.simpleGaugeMetricMap[name].metric
}

func (md *metricsDelegate) GetTimestampMetric(name metricName) *prometheus.GaugeVec {
	return MetricsExp.internalData.timestampMetricMap[name].metric
}

// GetFunctionMetrics returns a functionMetric for use if it exists, otherwise returns nil.
func GetFunctionMetrics(name metricName) (*FunctionMetrics, error) {
	returnVal, found := MetricsExp.internalData.functionMetricsMap[name]
	if !found {
		return returnVal, fmt.Errorf("%v is not a valid function metric, it is not in the functionMetrics map", name)
	}
	return returnVal, nil
}

func (md *metricsDelegate) GetFunctionTimestampMetric(name metricName) *prometheus.GaugeVec {
	return MetricsExp.internalData.functionMetricsMap[name].lastCallTimestamp.metric
}

func (md *metricsDelegate) GetFunctionDurationMetric(name metricName) prometheus.Summary {
	return MetricsExp.internalData.functionMetricsMap[name].durationMetric.metric
}

func (md *metricsDelegate) GetFunctionErrorMetric(name metricName) *prometheus.CounterVec {
	return MetricsExp.internalData.functionMetricsMap[name].errorTotal.metric
}

func (md *metricsDelegate) GetFunctionCounterMetric(name metricName) prometheus.Counter {
	return MetricsExp.internalData.functionMetricsMap[name].callsTotal.metric
}

// GetCounterMetrics returns a simpleCounterMetric for use if it exists, otherwise returns nil.
func GetCounterMetrics(name metricName) (*CounterMetric, error) {
	returnVal, found := MetricsExp.internalData.simpleCounterMetricMap[name]
	if !found {
		return returnVal, fmt.Errorf("%v is not a valid function metric, it is not in the simpleCounterMetric map", name)
	}
	return returnVal, nil
}

// GetGaugeMetrics returns a simpleGaugeMetric for use if it exists, otherwise returns nil.
func GetGaugeMetrics(name metricName) (*GaugeMetric, error) {
	returnVal, found := MetricsExp.internalData.simpleGaugeMetricMap[name]
	if !found {
		return returnVal, fmt.Errorf("%v is not a valid function metric, it is not in the simpleGaugeMetric map", name)
	}
	return returnVal, nil
}

// GetErrorMetrics returns a ErrorMetric for use if it exists, otherwise returns nil.
func GetErrorMetrics(name metricName) (*ErrorMetric, error) {
	returnVal, found := MetricsExp.internalData.errorMetricMap[name]
	if !found {
		return returnVal, fmt.Errorf("%v is not a valid function metric, it is not in the errorMetric map", name)
	}
	return returnVal, nil
}

// GetDurationMetrics returns a durationMetric for use if it exists, otherwise returns nil.
func GetDurationMetrics(name metricName) (*DurationMetric, error) {
	returnVal, found := MetricsExp.internalData.durationMetricMap[name]
	if !found {
		return returnVal, fmt.Errorf("%v is not a valid function metric, it is not in the durationMetric map", name)
	}
	return returnVal, nil
}

// GetTimestampMetrics returns a timeStampMetric for use if it exists, otherwise returns nil.
func GetTimestampMetrics(name metricName) (*TimestampMetric, error) {
	returnVal, found := MetricsExp.internalData.timestampMetricMap[name]
	if !found {
		return returnVal, fmt.Errorf("%v is not a valid function metric, it is not in the timestampMetric map", name)
	}
	return returnVal, nil
}
