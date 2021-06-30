// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package resources

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"gopkg.in/yaml.v2"

	vmov1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
)

func TestGetDefaultPrometheusConfiguration(t *testing.T) {
	vmi := &vmov1.VerrazzanoMonitoringInstance{}
	configText := GetDefaultPrometheusConfiguration(vmi)
	var config map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(configText), &config)
	if err != nil {
		t.Fatalf("Error parsing PrometheusConfiguration yaml %v", err)
	}
	scrapeConfigs := config["scrape_configs"]
	cadvisor := getItem("job_name", "cadvisor", scrapeConfigs.([]interface{}))
	kubernetesSdConfigs := cadvisor["kubernetes_sd_configs"]
	role := kubernetesSdConfigs.([]interface{})[0].(map[interface{}]interface{})["role"]
	assert.Equal(t, "node", role, "kubernetes_sd_configs should have - role: node")
	relabelConfigs := cadvisor["relabel_configs"]
	relabelConfig := getItem("target_label", "__metrics_path__", relabelConfigs.([]interface{}))
	assert.Equal(t, "__meta_kubernetes_node_name", relabelConfig["source_labels"].([]interface{})[0], "relabelConfig.source_labels")
	assert.Equal(t, "/api/v1/nodes/$1/proxy/metrics/cadvisor", relabelConfig["replacement"], "relabelConfig.replacement")

	pilot := getItem("job_name", "pilot", scrapeConfigs.([]interface{}))
	assert.NotNil(t, pilot)
	kubernetesSdConfigs = pilot["kubernetes_sd_configs"]
	role = kubernetesSdConfigs.([]interface{})[0].(map[interface{}]interface{})["role"]
	assert.Equal(t, "endpoints", role, "kubernetes_sd_configs should have - role: endpoints")

	envoyStats := getItem("job_name", "envoy-stats", scrapeConfigs.([]interface{}))
	assert.NotNil(t, envoyStats)
	kubernetesSdConfigs = envoyStats["kubernetes_sd_configs"]
	role = kubernetesSdConfigs.([]interface{})[0].(map[interface{}]interface{})["role"]
	assert.Equal(t, "pod", role, "kubernetes_sd_configs should have - role: pod")

	ingressController := getItem("job_name", "nginx-ingress-controller", scrapeConfigs.([]interface{}))
	assert.NotNil(t, ingressController)
	kubernetesSdConfigs = ingressController["kubernetes_sd_configs"]
	role = kubernetesSdConfigs.([]interface{})[0].(map[interface{}]interface{})["role"]
	assert.Equal(t, "pod", role, "kubernetes_sd_configs should have - role: pod")
	relabelConfigs = ingressController["relabel_configs"]
	relabelConfig = getItem("target_label", "__address__", relabelConfigs.([]interface{}))
	assert.Equal(t, "$1:10254", relabelConfig["replacement"], "relabelConfig.replacement")
}

func getItem(key, value string, scrapeConfigs []interface{}) map[interface{}]interface{} {
	for _, sc := range scrapeConfigs {
		config := sc.(map[interface{}]interface{})
		if config[key] == value {
			return config
		}
	}
	return nil
}

func TestIsSingleNodeESCluster(t *testing.T) {
	vmo := &vmov1.VerrazzanoMonitoringInstance{
		Spec: vmov1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			Grafana: vmov1.Grafana{
				Enabled: true,
			},
			Prometheus: vmov1.Prometheus{
				Enabled:  true,
				Replicas: 1,
			},
			AlertManager: vmov1.AlertManager{
				Enabled: true,
			},
			Kibana: vmov1.Kibana{
				Enabled: true,
			},
			Elasticsearch: vmov1.Elasticsearch{
				Enabled:    true,
				IngestNode: vmov1.ElasticsearchNode{Replicas: 0},
				MasterNode: vmov1.ElasticsearchNode{Replicas: 1},
				DataNode:   vmov1.ElasticsearchNode{Replicas: 0},
			},
		},
	}
	assert.True(t, IsSingleNodeESCluster(vmo), "IsSingleNodeCluster false for valid configuration")

	vmo.Spec.Elasticsearch.MasterNode.Replicas = 2
	assert.False(t, IsSingleNodeESCluster(vmo), "IsSingleNodeCluster true for invalid configuration")

	vmo.Spec.Elasticsearch.MasterNode.Replicas = 1
	vmo.Spec.Elasticsearch.IngestNode.Replicas = 1
	assert.False(t, IsSingleNodeESCluster(vmo), "IsSingleNodeCluster true for invalid configuration")
}

func TestIsValidMultiNodeESCluster(t *testing.T) {
	vmo := &vmov1.VerrazzanoMonitoringInstance{
		Spec: vmov1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			Grafana: vmov1.Grafana{
				Enabled: true,
			},
			Prometheus: vmov1.Prometheus{
				Enabled:  true,
				Replicas: 1,
			},
			AlertManager: vmov1.AlertManager{
				Enabled: true,
			},
			Kibana: vmov1.Kibana{
				Enabled: true,
			},
			Elasticsearch: vmov1.Elasticsearch{
				Enabled:    true,
				IngestNode: vmov1.ElasticsearchNode{Replicas: 0},
				MasterNode: vmov1.ElasticsearchNode{Replicas: 1},
				DataNode:   vmov1.ElasticsearchNode{Replicas: 0},
			},
		},
	}
	assert.False(t, IsValidMultiNodeESCluster(vmo), "IsValidMultiNodeESCluster true for single-node configuration")

	vmo.Spec.Elasticsearch.MasterNode.Replicas = 1
	vmo.Spec.Elasticsearch.DataNode.Replicas = 1
	vmo.Spec.Elasticsearch.IngestNode.Replicas = 1
	assert.True(t, IsValidMultiNodeESCluster(vmo), "IsValidMultiNodeESCluster false for valid configuration")

	vmo.Spec.Elasticsearch.MasterNode.Replicas = 3
	vmo.Spec.Elasticsearch.DataNode.Replicas = 2
	vmo.Spec.Elasticsearch.IngestNode.Replicas = 1
	assert.True(t, IsValidMultiNodeESCluster(vmo), "IsValidMultiNodeESCluster true for valid configuration")
}

//func TestGetMaxHeap(t *testing.T) {
//	GetMaxHeap("1Mi")
//	GetMaxHeap("15Mi")
//	GetMaxHeap("160Mi")
//	GetMaxHeap("1700Mi")
//	GetMaxHeap("18000Mi")
//	GetMaxHeap("19000Mi")
//	GetMaxHeap(".15Gi")
//	GetMaxHeap("7.5Gi")
//	GetMaxHeap("10Gi")
//	GetMaxHeap("100Gi")
//	GetMaxHeap("1000Gi")
//}

func TestGetMaxHeap(t *testing.T) {

	tests := []struct {
		name string
		size string
		expected string
	}{
		{name: "test-K-1", size: "1Ki", expected: "1k",},
		{name: "test-K-2", size: "1.1Ki", expected: "1127",},
		{name: "test-K-3", size: "1500Ki", expected: "1500k",},
		{name: "test-K-4", size: "1024Ki", expected: "1m",},
		{name: "test-K-4", size: ".5Ki", expected: "512",},

		{name: "test-M-1", size: "1Mi", expected: "1m",},
		{name: "test-M-2", size: "1.1Mi", expected: "1153434",},
		{name: "test-M-3", size: "1500Mi", expected: "1500m",},
		{name: "test-M-4", size: "1024Mi", expected: "1g",},
		{name: "test-M-5", size: ".5Mi", expected: "512k",},

		{name: "test-G-1", size: "1Gi", expected: "1g",},
		{name: "test-G-2", size: "1.1Gi", expected: "1181116007",},
		{name: "test-G-3", size: "1500Gi", expected: "1500g",},
		{name: "test-G-4", size: "1024Gi", expected: "1024g",},
		{name: "test-G-5", size: ".5Gi", expected: "512m",},

	}
		for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			asserts := assert.New(t)
			n, err := ConvertPodMemToJvmHeap(test.size)
			asserts.NoError(err, "GetMaxHeap returned error")
			asserts.Equal(test.expected, n, fmt.Sprintf("%s failed",test.name))
		})
	}
}

// TestFormatJvmHeapSize tests the formatting of values using 1024 multiples
// GIVEN a integer ranging from 100 to over 1G
// WHEN formatJvmHeapSize is called
// THEN ensure the result has whole numbers and correct suffix is returned: K, M, or G suffix.
func TestFormatJvmHeapSize(t *testing.T) {
	asserts := assert.New(t)
	asserts.Equal("100", formatJvmHeapSize(100), "expected 100")
	asserts.Equal("1k", formatJvmHeapSize(UnitK), "expected 1k")
	asserts.Equal("10k", formatJvmHeapSize(10 * UnitK), "expected 10k")
	asserts.Equal("1000k", formatJvmHeapSize(1000 * UnitK), "expected 1000k")
	asserts.Equal("10000k", formatJvmHeapSize(10000 * UnitK), "expected 10000k")
	asserts.Equal("1m", formatJvmHeapSize(UnitK * UnitK), "expected 1m")

	asserts.Equal("1m", formatJvmHeapSize(UnitM), "expected 1m")
	asserts.Equal("10m", formatJvmHeapSize(10 * UnitM), "expected 10m")
	asserts.Equal("1000m", formatJvmHeapSize(1000 * UnitM), "expected 1000m")
	asserts.Equal("10000m", formatJvmHeapSize(10000 * UnitM), "expected 10000m")
	asserts.Equal("1g", formatJvmHeapSize(UnitK * UnitM), "expected 1g")

	asserts.Equal("1g", formatJvmHeapSize(UnitG), "expected 1g")
	asserts.Equal("10g", formatJvmHeapSize(10 * UnitG), "expected 10g")
	asserts.Equal("1000g", formatJvmHeapSize(1000 * UnitG), "expected 1000g")
	asserts.Equal("10000g", formatJvmHeapSize(10000 * UnitG), "expected 10000g")
	asserts.Equal("1024g", formatJvmHeapSize(UnitK * UnitG), "expected 1024g")
}
