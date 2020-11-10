// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package config

import (
	"fmt"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
)

// NewConfigFromConfigMap creates a new OperatorConfig from the given ConfigMap,
func NewConfigFromConfigMap(configMap *corev1.ConfigMap) (*OperatorConfig, error) {
	// Parse configMap content and unmarshall into OperatorConfig struct
	zap.S().Infow("Constructing config from config map")
	var configString string
	if value, ok := configMap.Data[configKeyValue]; ok {
		configString = value
	} else {
		return nil, fmt.Errorf("expected key '%s' not found in ConfigMap %s", configKeyValue, configMap.Name)
	}
	var config OperatorConfig
	err := yaml.Unmarshal([]byte(configString), &config)
	zap.S().Debugf("Unmarshalled configmap is:\n %s", configMap.String())
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall ConfigMap %s: %v", configMap.String(), err)
	}

	// Set defaults for any uninitialized values
	zap.S().Infow("Setting config defaults")
	setConfigDefaults(&config)
	return &config, nil
}

// Sets defaults for the given OperatorConfig object.
func setConfigDefaults(config *OperatorConfig) {

	if config.DefaultSimpleComponentReplicas == nil {
		config.DefaultSimpleComponentReplicas = newIntVal(defaultSimpleComponentReplicas)
	}
	if config.DefaultPrometheusReplicas == nil {
		config.DefaultPrometheusReplicas = newIntVal(defaultPrometheusReplicas)
	}
	if config.MetricsPort == nil {
		config.MetricsPort = newIntVal(defaultMetricsPort)
	}

}

func newIntVal(value int) *int {
	var val = value
	return &val
}
