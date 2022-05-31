// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package config

// OperatorConfig type for operator configuration
type OperatorConfig struct {
	EnvName                        string   `yaml:"envName"`
	DefaultIngressTargetDNSName    string   `yaml:"defaultIngressTargetDNSName,omitempty"`
	DefaultSimpleComponentReplicas *int     `yaml:"defaultSimpleCompReplicas"`
	MetricsPort                    *int     `yaml:"metricsPort"`
	NatGatewayIPs                  []string `yaml:"natGatewayIPs"`
	Pvcs                           Pvcs     `yaml:"pvcs"`
}

// Pvcs type for storage
type Pvcs struct {
	StorageClass   string `yaml:"storageClass"`
	ZoneMatchLabel string `yaml:"zoneMatchLabel"`
}

// DefaultOperatorConfigmapName config map name for operator
const DefaultOperatorConfigmapName = "verrazzano-monitoring-operator-config"

const configKeyValue = "config"
const defaultSimpleComponentReplicas = 1
const defaultMetricsPort = 8090
