// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// ComponentDetails struct for component detail info
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
	OidcProxy         *ComponentDetails
	Optional          bool
	Disabled          bool
}

// AllComponentDetails is array of all ComponentDetails
var AllComponentDetails = []*ComponentDetails{&Grafana, &Prometheus, &PrometheusInit, &AlertManager, &AlertManagerCluster, &Kibana, &ElasticsearchIngest, &ElasticsearchMaster, &ElasticsearchData, &ElasticsearchInit, &API, &ConfigReloader, &OidcProxy}

// StorageEnableComponents is storage operation-related stuff
var StorageEnableComponents = []*ComponentDetails{&Grafana, &Prometheus, &ElasticsearchData}

// Grafana is the default Grafana configuration
var Grafana = ComponentDetails{
	Name:              "grafana",
	EnvName:           "GRAFANA_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              3000,
	DataDir:           "/var/lib/grafana",
	LivenessHTTPPath:  "/api/health",
	ReadinessHTTPPath: "/api/health",
	Privileged:        false,
	OidcProxy:         &OidcProxy,
}

// Prometheus is the default Prometheus configuration
// Note: Update promtool version to match any version changes here
//    - vmo/images/cirith-server-for-operator/docker-images
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
	OidcProxy:         &OidcProxy,
}

// PrometheusInit is the default Prometheus InitContainer configuration
var PrometheusInit = ComponentDetails{
	Name:            "prometheus-init",
	EnvName:         "PROMETHEUS_INIT_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
	DataDir:         "/prometheus",
}

// AlertManager is the default AlertManager configuration
// Note: Update amtool version to match any version changes here
//   - vmo/images/cirith-server-for-operator/docker-images
var AlertManager = ComponentDetails{
	Name:              "alertmanager",
	EnvName:           "ALERT_MANAGER_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              9093,
	LivenessHTTPPath:  "/-/healthy",
	ReadinessHTTPPath: "/-/ready",
	Privileged:        false,
}

// AlertManagerCluster is AlertManager cluster settings - used in standalone AlertManager cluster service
var AlertManagerCluster = ComponentDetails{
	Name: "alertmanager-cluster",
	Port: 9094,
}

// ESWait is the InitContainer config; will wait for ES to reach stable healthy state
var ESWait = ComponentDetails{
	Name:            "wait-for-es",
	EnvName:         "ELASTICSEARCH_WAIT_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
}

// Kibana is the default Kibana configuration
var Kibana = ComponentDetails{
	Name:              "kibana",
	EnvName:           "KIBANA_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              5601,
	LivenessHTTPPath:  "/api/status",
	ReadinessHTTPPath: "/api/status",
	Privileged:        false,
	OidcProxy:         &OidcProxy,
}

// OidcProxy is the default OIDC proxy configuration
var OidcProxy = ComponentDetails{
	Name:            "oidc",
	EnvName:         "OIDC_PROXY_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
	Port:            constants.OidcProxyPort,
}

// ElasticsearchIngest is the default Elasticsearch Ingest configuration
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
	OidcProxy:         &OidcProxy,
}

// ElasticsearchMaster is the default Elasticsearch Master configuration
var ElasticsearchMaster = ComponentDetails{
	Name:            "es-master",
	EnvName:         "ELASTICSEARCH_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
	Port:            constants.ESTransportPort,
	Privileged:      false,
}

// ElasticsearchData is the default Elasticsearch Data configuration
var ElasticsearchData = ComponentDetails{
	Name:              "es-data",
	EnvName:           "ELASTICSEARCH_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              constants.ESHttpPort,
	DataDir:           "/usr/share/opensearch/data",
	LivenessHTTPPath:  "/_cluster/health",
	ReadinessHTTPPath: "/_cluster/health",
	Privileged:        false,
}

// ElasticsearchInit contains Elasticsearch init container info
var ElasticsearchInit = ComponentDetails{
	Name:            "elasticsearch-init",
	EnvName:         "ELASTICSEARCH_INIT_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
	Privileged:      true,
}

// API is the default API configuration
var API = ComponentDetails{
	Name:              "api",
	EnvName:           "VERRAZZANO_MONITORING_INSTANCE_API_IMAGE",
	ImagePullPolicy:   constants.DefaultImagePullPolicy,
	Port:              9097,
	LivenessHTTPPath:  "/healthcheck",
	ReadinessHTTPPath: "/healthcheck",
	Privileged:        false,
	Optional:          true,
}

// ConfigReloader is the default config-reloader configuration
var ConfigReloader = ComponentDetails{
	Name:            "config-reloader",
	EnvName:         "CONFIG_RELOADER_IMAGE",
	ImagePullPolicy: constants.DefaultImagePullPolicy,
	Privileged:      false,
}

const (
	eswaitTargetVersionEnv = "ELASTICSEARCH_WAIT_TARGET_VERSION"
	oidcAuthEnabled        = "OIDC_AUTH_ENABLED"
)

// ESWaitTargetVersion contains value for environment variable ELASTICSEARCH_WAIT_TARGET_VERSION
var ESWaitTargetVersion string

// InitComponentDetails initialize all components and check ELASTICSEARCH_WAIT_TARGET_VERSION
func InitComponentDetails() error {
	//oidcAuthEnabled defaults to true
	oidcAuthEnabled := !strings.EqualFold("false", os.Getenv(oidcAuthEnabled))
	// Initialize the images to use
	for _, component := range AllComponentDetails {
		if len(component.EnvName) > 0 {
			component.Image = os.Getenv(component.EnvName)
			if len(component.Image) == 0 {
				if !component.Optional {
					return fmt.Errorf("Failed, the environment variable %s translated to an empty string for component %s", component.EnvName, component.Name)
				}
				// if no image is provided for an optional component then disable it
				zap.S().Infof("The environment variable %s translated to an empty string for optional component %s.  Marking component disabled.", component.EnvName, component.Name)
				component.Disabled = true
			}
		}
		if !oidcAuthEnabled {
			component.OidcProxy = nil
		}
	}
	ESWaitTargetVersion = os.Getenv(eswaitTargetVersionEnv)
	if len(ESWaitTargetVersion) == 0 {
		return fmt.Errorf("Failed, the environment variable %s translated to an empty string", eswaitTargetVersionEnv)
	}
	return nil
}
