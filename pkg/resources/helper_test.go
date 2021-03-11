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
