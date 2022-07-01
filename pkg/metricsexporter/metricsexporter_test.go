// Copyright (c) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package metricsexporter

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

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
