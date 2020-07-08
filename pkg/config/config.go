// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package config

import (
	"fmt"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"os"
)

// Creates a new OperatorConfig from the given ConfigMap,
func NewConfigFromConfigMap(configMap *corev1.ConfigMap) (*OperatorConfig, error) {
	//create log for new config
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "ConfigMap").Str("name", configMap.Name).Logger()

	// Parse configMap content and unmarshall into OperatorConfig struct
	logger.Info().Msg("Constructing config from config map")
	var configString string
	if value, ok := configMap.Data[ConfigKeyValue]; ok {
		configString = value
	} else {
		return nil, fmt.Errorf("expected key '%s' not found in ConfigMap %s", ConfigKeyValue, configMap.Name)
	}
	var config OperatorConfig
	err := yaml.Unmarshal([]byte(configString), &config)
	logger.Debug().Msgf("Unmarshalled configmap is:\n %s", configMap.String())
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall ConfigMap %s: %v", configMap.String(), err)
	}

	// Set defaults for any uninitialized values
	logger.Info().Msg("Setting config defaults")
	setConfigDefaults(&config)
	return &config, nil
}

// Sets defaults for the given OperatorConfig object.
func setConfigDefaults(config *OperatorConfig) {

	if config.DefaultSimpleComponentReplicas == nil {
		config.DefaultSimpleComponentReplicas = newIntVal(DefaultSimpleComponentReplicas)
	}
	if config.DefaultPrometheusReplicas == nil {
		config.DefaultPrometheusReplicas = newIntVal(DefaultPrometheusReplicas)
	}
	if config.MetricsPort == nil {
		config.MetricsPort = newIntVal(DefaultMetricsPort)
	}

}

func newIntVal(value int) *int {
	var val = value
	return &val
}
