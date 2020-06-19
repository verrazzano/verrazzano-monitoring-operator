// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package config

type OperatorConfig struct {
	EnvName                        string   `yaml:"envName"`
	DefaultIngressTargetDNSName    string   `yaml:"defaultIngressTargetDNSName,omitempty"`
	DefaultSimpleComponentReplicas *int     `yaml:"defaultSimpleCompReplicas"`
	DefaultPrometheusReplicas      *int     `yaml:"defaultPrometheusReplicas"`
	CompartmentId                  string   `yaml:"compartmentId,omitempty"`
	MetricsPort                    *int     `yaml:"metricsPort"`
	NatGatewayIPs                  []string `yaml:"natGatewayIPs"`
	Pvcs                           Pvcs     `yaml:"pvcs"`
}

type Pvcs struct {
	StorageClass   string `yaml:"storageClass"`
	ZoneMatchLabel string `yaml:"zoneMatchLabel"`
}

const ConfigKeyValue = "config"
const DefaultOperatorConfigmapName = "verrazzano-monitoring-operator-config"
const DefaultSimpleComponentReplicas = 1
const DefaultPrometheusReplicas = 3
const DefaultMetricsPort = 8090
