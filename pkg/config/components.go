// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
)

type ComponentDetails struct {
	Name              string
	EndpointName      string
	Image             string
	ImagePullPolicy   corev1.PullPolicy
	Port              int
	DataDir           string
	LivenessHTTPPath  string
	ReadinessHTTPPath string
	Privileged        bool
	RunAsUser         int64
	EnvName           string
}

// Array of all ComponentDetails
var AllComponentDetails = []*ComponentDetails{&Grafana, &Prometheus, &PrometheusInit, &PrometheusGW, &AlertManager, &AlertManagerCluster, &ESWait, &Kibana, &ElasticsearchIngest, &ElasticsearchMaster, &ElasticsearchData, &ElasticsearchInit, &Api, &ElasticsearchExporter, &ConfigReloader, &NodeExporter}

// Storage operation-related stuff
var StorageEnableComponents = []*ComponentDetails{&Grafana, &Prometheus, &ElasticsearchData}

// Default Grafana configuration
var Grafana = ComponentDetails{
	Name:              "grafana",
	EnvName:           "GRAFANA_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              3000,
	DataDir:           "/var/lib/grafana",
	LivenessHTTPPath:  "/api/health",
	ReadinessHTTPPath: "/api/health",
	Privileged:        false,
}

// Default Prometheus configuration
// Note: Update promtool version to match any version changes here
//    - sauron/images/cirith-server-for-operator/docker-images
var Prometheus = ComponentDetails{
	Name:              "prometheus",
	EnvName:           "PROMETHEUS_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              9090,
	DataDir:           "/prometheus",
	LivenessHTTPPath:  "/-/healthy",
	ReadinessHTTPPath: "/-/ready",
	Privileged:        false,
	RunAsUser:         int64(constants.NobodyUID),
}

// Default Prometheus InitContainer configuration
var PrometheusInit = ComponentDetails{
	Name:            "prometheus-init",
	EnvName:         "PROMETHEUS_INIT_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
	DataDir:         "/prometheus",
}

// Default Prometheus Push Gateway configuration
var PrometheusGW = ComponentDetails{
	Name:              "prometheus-gw",
	EnvName:           "PROMETHEUS_GATEWAY_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              9091,
	LivenessHTTPPath:  "/-/healthy",
	ReadinessHTTPPath: "/-/ready",
	Privileged:        false,
}

// Default AlertManager configuration
// Note: Update amtool version to match any version changes here
//   - sauron/images/cirith-server-for-operator/docker-images
var AlertManager = ComponentDetails{
	Name:              "alertmanager",
	EnvName:           "ALERT_MANAGER_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              9093,
	LivenessHTTPPath:  "/-/healthy",
	ReadinessHTTPPath: "/-/ready",
	Privileged:        false,
}

// AlertManager cluster settings - used in standalone AlertManager cluster service
var AlertManagerCluster = ComponentDetails{
	Name: "alertmanager-cluster",
	Port: 9094,
}

// InitContainer config; will wait for ES to reach stable healthy state
var ESWait = ComponentDetails{
	Name:            "wait-for-es",
	EnvName:         "ELASTICSEARCH_WAIT_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
}

// Default Kibana configuration
var Kibana = ComponentDetails{
	Name:              "kibana",
	EnvName:           "KIBANA_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              5601,
	LivenessHTTPPath:  "/api/status",
	ReadinessHTTPPath: "/api/status",
	Privileged:        false,
}

// Default Elasticsearch Ingest configuration
var ElasticsearchIngest = ComponentDetails{
	Name:         "es-ingest",
	EndpointName: "elasticsearch",
	//NOTE: update ELASTICSEARCH_WAIT_TARGET_VERSION env (constants.ESWaitTargetVersionEnv) value to match the version reported by this image
	EnvName:           "ELASTICSEARCH_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              constants.ESHttpPort,
	LivenessHTTPPath:  "/_cluster/health",
	ReadinessHTTPPath: "/_cluster/health",
	Privileged:        false,
}

// Default Elasticsearch Master configuration
var ElasticsearchMaster = ComponentDetails{
	Name:            "es-master",
	EnvName:         "ELASTICSEARCH_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
	Port:            constants.ESTransportPort,
	Privileged:      false,
}

// Default Elasticsearch Data configuration
var ElasticsearchData = ComponentDetails{
	Name:              "es-data",
	EnvName:           "ELASTICSEARCH_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              constants.ESHttpPort,
	DataDir:           "/usr/share/elasticsearch/data",
	LivenessHTTPPath:  "/_cluster/health",
	ReadinessHTTPPath: "/_cluster/health",
	Privileged:        false,
}

// Elasticsearch init container
var ElasticsearchInit = ComponentDetails{
	Name:            "elasticsearch-init",
	EnvName:         "ELASTICSEARCH_INIT_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
	Privileged:      true,
}

// Default Api configuration
var Api = ComponentDetails{
	Name:              "api",
	EnvName:           "VERRAZZANO_MONITORING_INSTANCE_API_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              9097,
	LivenessHTTPPath:  "/healthcheck",
	ReadinessHTTPPath: "/healthcheck",
	Privileged:        false,
}

// Default ElasticsearchExporter configuration
var ElasticsearchExporter = ComponentDetails{
	Name:              "es-exporter",
	EnvName:           "ELASTICSEARCH_EXPORTER_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              9114,
	LivenessHTTPPath:  "/",
	ReadinessHTTPPath: "/",
	Privileged:        true,
}

// Default config-reloader configuration
var ConfigReloader = ComponentDetails{
	Name:            "config-reloader",
	EnvName:         "CONFIG_RELOADER_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
	Privileged:      false,
}

// Default node-exporter configuration
var NodeExporter = ComponentDetails{
	Name:            "node-exporter",
	EnvName:         "NODE_EXPORTER_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
	Port:            9100,
	Privileged:      true,
}

const ESWaitTargetVersionEnv = "ELASTICSEARCH_WAIT_TARGET_VERSION"

var ESWaitTargetVersion string

func InitComponentDetails() error {
	// Initialize the images to use
	for _, component := range AllComponentDetails {
		if len(component.EnvName) > 0 {
			component.Image = os.Getenv(component.EnvName)
			if len(component.Image) == 0 {
				return errors.New(fmt.Sprintf("The environment variable %s translated to an empty string for component %s", component.EnvName, component.Name))
			}
		}
	}
	ESWaitTargetVersion = os.Getenv(ESWaitTargetVersionEnv)
	if len(ESWaitTargetVersion) == 0 {
		return errors.New(fmt.Sprintf("The environment variable %s translated to an empty string", ESWaitTargetVersionEnv))
	}
	return nil
}
