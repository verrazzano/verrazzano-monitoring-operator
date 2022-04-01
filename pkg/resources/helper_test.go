// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package resources

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"

	"gopkg.in/yaml.v2"

	vmov1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
)

func TestGetDefaultPrometheusConfiguration(t *testing.T) {
	vmi := &vmov1.VerrazzanoMonitoringInstance{}
	configText := GetDefaultPrometheusConfiguration(vmi, "myclustername")
	var config map[interface{}]interface{}
	err := yaml.Unmarshal([]byte(configText), &config)
	if err != nil {
		t.Fatalf("Error parsing PrometheusConfiguration yaml %v", err)
	}
	scrapeConfigs := config["scrape_configs"]

	prometheus := getItem("job_name", "prometheus", scrapeConfigs.([]interface{}))
	assert.NotNil(t, prometheus)
	staticConfigs := prometheus["static_configs"]
	assert.NotNil(t, staticConfigs)
	staticCfg := staticConfigs.([]interface{})[0].(map[interface{}]interface{})
	assert.Equal(t, "localhost:9090", staticCfg["targets"].([]interface{})[0])
	staticLabels := staticCfg["labels"].(map[interface{}]interface{})
	assert.Equal(t, "myclustername", staticLabels[constants.PrometheusClusterNameLabel])

	nodeExporter := getItem("job_name", "node-exporter", scrapeConfigs.([]interface{}))
	assert.NotNil(t, nodeExporter)
	kubernetesSdConfigs := nodeExporter["kubernetes_sd_configs"]
	role := kubernetesSdConfigs.([]interface{})[0].(map[interface{}]interface{})["role"]
	assert.Equal(t, "endpoints", role, "kubernetes_sd_configs for node-exporter should have - role: endpoints")
	assertVzClusterNameRelabelConfig(t, nodeExporter["relabel_configs"], "myclustername")

	cadvisor := getItem("job_name", "cadvisor", scrapeConfigs.([]interface{}))
	kubernetesSdConfigs = cadvisor["kubernetes_sd_configs"]
	role = kubernetesSdConfigs.([]interface{})[0].(map[interface{}]interface{})["role"]
	assert.Equal(t, "node", role, "kubernetes_sd_configs for cadvisor should have - role: node")
	relabelConfigs := cadvisor["relabel_configs"]
	relabelConfig := getItem("target_label", "__metrics_path__", relabelConfigs.([]interface{}))
	assert.Equal(t, "__meta_kubernetes_node_name", relabelConfig["source_labels"].([]interface{})[0], "relabelConfig.source_labels")
	assert.Equal(t, "/api/v1/nodes/$1/proxy/metrics/cadvisor", relabelConfig["replacement"], "relabelConfig.replacement")
	assertVzClusterNameRelabelConfig(t, relabelConfigs, "myclustername")

	pilot := getItem("job_name", "pilot", scrapeConfigs.([]interface{}))
	assert.NotNil(t, pilot)
	kubernetesSdConfigs = pilot["kubernetes_sd_configs"]
	role = kubernetesSdConfigs.([]interface{})[0].(map[interface{}]interface{})["role"]
	assert.Equal(t, "endpoints", role, "kubernetes_sd_configs should have - role: endpoints")
	assertVzClusterNameRelabelConfig(t, pilot["relabel_configs"], "myclustername")

	envoyStats := getItem("job_name", "envoy-stats", scrapeConfigs.([]interface{}))
	assert.NotNil(t, envoyStats)
	kubernetesSdConfigs = envoyStats["kubernetes_sd_configs"]
	role = kubernetesSdConfigs.([]interface{})[0].(map[interface{}]interface{})["role"]
	assert.Equal(t, "pod", role, "kubernetes_sd_configs should have - role: pod")
	assertVzClusterNameRelabelConfig(t, envoyStats["relabel_configs"], "myclustername")

	ingressController := getItem("job_name", "nginx-ingress-controller", scrapeConfigs.([]interface{}))
	assert.NotNil(t, ingressController)
	kubernetesSdConfigs = ingressController["kubernetes_sd_configs"]
	role = kubernetesSdConfigs.([]interface{})[0].(map[interface{}]interface{})["role"]
	assert.Equal(t, "pod", role, "kubernetes_sd_configs should have - role: pod")
	relabelConfigs = ingressController["relabel_configs"]
	relabelConfig = getItem("target_label", "__address__", relabelConfigs.([]interface{}))
	assert.Equal(t, "$1:10254", relabelConfig["replacement"], "relabelConfig.replacement")
	assertVzClusterNameRelabelConfig(t, ingressController["relabel_configs"], "myclustername")
}

// asserts that the relabel config for adding the verrazzano cluster name label to the metric exists and is
// as expected
func assertVzClusterNameRelabelConfig(t *testing.T, relabelConfigs interface{}, expectedClusterName string) {
	clusterNameRelabelConfig := getItem("target_label", constants.PrometheusClusterNameLabel, relabelConfigs.([]interface{}))
	assert.NotNil(t, clusterNameRelabelConfig)
	assert.Equal(t, "replace", clusterNameRelabelConfig["action"])
	assert.Equal(t, expectedClusterName, clusterNameRelabelConfig["replacement"])
	assert.Equal(t, nil, clusterNameRelabelConfig["source_labels"])
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

func TestGetOpenSearchDashboardsHTTPEndpoint(t *testing.T) {
	vmi := &vmov1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "system",
		},
	}

	osdEndpoint := GetOpenSearchDashboardsHTTPEndpoint(vmi)
	assert.Equal(t, "http://vmi-system-kibana:5601", osdEndpoint)
}

func TestGetOpenSearchHTTPEndpoint(t *testing.T) {
	vmi := &vmov1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "system",
		},
	}

	osEndpoint := GetOpenSearchHTTPEndpoint(vmi)
	assert.Equal(t, "http://vmi-system-es-master-http:9200", osEndpoint)
}

func TestConvertToRegexp(t *testing.T) {
	var tests = []struct {
		pattern string
		regexp  string
	}{
		{
			"verrazzano-*",
			"^verrazzano-.*$",
		},
		{
			"verrazzano-system",
			"^verrazzano-system$",
		},
		{
			"*",
			"^.*$",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("converting pattern '%s' to regexp", tt.pattern), func(t *testing.T) {
			r := ConvertToRegexp(tt.pattern)
			assert.Equal(t, tt.regexp, r)
		})
	}
}
