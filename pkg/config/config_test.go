// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
)

func TestNewConfigFromConfigMap(t *testing.T) {
	configStr := `envName: testenv`
	fmt.Printf("test string:\n %s", configStr)

	operatorConfig, err := CreateConfigFromStr(configStr)
	if err != nil {
		t.Fatalf("Failed to parse configmap into config: %v", err)
	}

	assert.Equal(t, *operatorConfig.MetricsPort, 8090)

}

func TestConfigDefaults(t *testing.T) {
	configStr := `envName: testenv`

	operatorConfig, err := CreateConfigFromStr(configStr)
	if err != nil {
		t.Fatalf("Failed to parse configmap into config: %v", err)
	}

	assert.Equal(t, *operatorConfig.MetricsPort, 8090)
	assert.Equal(t, *operatorConfig.DefaultSimpleComponentReplicas, DefaultSimpleComponentReplicas)
	assert.Equal(t, *operatorConfig.DefaultPrometheusReplicas, DefaultPrometheusReplicas)
	assert.Equal(t, operatorConfig.DefaultIngressTargetDNSName, "")
}

func CreateConfigFromStr(configStr string) (*OperatorConfig, error) {
	configMap := corev1.ConfigMap{}
	configMap.Data = map[string]string{ConfigKeyValue: configStr}
	operatorConfig, err := NewConfigFromConfigMap(&configMap)
	return operatorConfig, err
}
