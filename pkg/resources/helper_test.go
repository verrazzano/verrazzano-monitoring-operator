// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package resources

import (
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

func TestGetMaxHeap(t *testing.T) {
	GetMaxHeap("1Mi")
	GetMaxHeap("15Mi")
	GetMaxHeap("160Mi")
	GetMaxHeap("1700Mi")
	GetMaxHeap("18000Mi")
	GetMaxHeap("19000Mi")
	GetMaxHeap(".15Gi")
	GetMaxHeap("7.5Gi")
	GetMaxHeap("10Gi")
	GetMaxHeap("100Gi")
	GetMaxHeap("1000Gi")
}

// TestFormatJvmHeapSize tests the formatting of Kilobyte values using 1024 multiples
// GIVEN a integer ranging from 1K to over 1G
// WHEN formatJvmHeapSize is called
// THEN ensure the heap in string with whole numbers and correct suffix is returned: K, M, or G suffix.
func TestFormatJvmHeapSize(t *testing.T) {
	asserts := assert.New(t)
	asserts.Equal("100", formatJvmHeapSize(100), "expected 100")
	asserts.Equal("1K", formatJvmHeapSize(UnitK), "expected 1K")
	asserts.Equal("10K", formatJvmHeapSize(10 * UnitK), "expected 10K")
	asserts.Equal("1000K", formatJvmHeapSize(1000 * UnitK), "expected 1000K")
	asserts.Equal("10000K", formatJvmHeapSize(10000 * UnitK), "expected 10000K")
	asserts.Equal("1M", formatJvmHeapSize(UnitK * UnitK), "expected 1M")

	asserts.Equal("1M", formatJvmHeapSize(UnitM), "expected 1M")
	asserts.Equal("10M", formatJvmHeapSize(10 * UnitM), "expected 10M")
	asserts.Equal("1000M", formatJvmHeapSize(1000 * UnitM), "expected 1000M")
	asserts.Equal("10000M", formatJvmHeapSize(10000 * UnitM), "expected 10000M")
	asserts.Equal("1G", formatJvmHeapSize(UnitK * UnitM), "expected 1G")

	asserts.Equal("1G", formatJvmHeapSize(UnitG), "expected 1G")
	asserts.Equal("10G", formatJvmHeapSize(10 * UnitG), "expected 10G")
	asserts.Equal("1000G", formatJvmHeapSize(1000 * UnitG), "expected 1000G")
	asserts.Equal("10000G", formatJvmHeapSize(10000 * UnitG), "expected 10000G")
	asserts.Equal("1024G", formatJvmHeapSize(UnitK * UnitG), "expected 1024G")
}
