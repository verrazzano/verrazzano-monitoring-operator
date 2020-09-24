// Copyright (C) 2020, Oracle and/or its affiliates.
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
