// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package constants

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

// DefaultImagePullPolicy default pull policy for images
const DefaultImagePullPolicy = corev1.PullIfNotPresent

// NobodyUID UID for user nobody
const NobodyUID = 65534

// VMOStatus string for status
type VMOStatus string

// Status values for VMO
const (
	Running = VMOStatus("Running")
)

// VMOGroup group name for an instance resource
const VMOGroup = "verrazzano.io"

// VMOVersion version of instance resource
const VMOVersion = "v1"

// VMOLabel label for an instance resource
const VMOLabel = "vmo." + VMOVersion + "." + VMOGroup

// VMOKind kind for an instance resource
const VMOKind = "VerrazzanoMonitoringInstance"

// VMOPlural plural name for an instance resource
const VMOPlural = "verrazzanomonitoringinstances"

// VMOFullname full name for an instance resource
const VMOFullname = VMOPlural + "." + VMOGroup

// ServiceAccountName service account name for VMO
const ServiceAccountName = "verrazzano-monitoring-operator"

// RoleBindingForVMOInstance rolebinding name for VMO instance
const RoleBindingForVMOInstance = "verrazzano-monitoring-operator"

// ClusterRoleForVMOInstances clusterrole name for VMO instance
const ClusterRoleForVMOInstances = "vmi-cluster-role"

// ResyncPeriod (re-list time period) for VMO Controller
const ResyncPeriod = 30 * time.Second

// VMOServiceNamePrefix to be applied to all VMO services
const VMOServiceNamePrefix = "vmi-"

// StorageVolumeName constant for storage volume
const StorageVolumeName = "storage-volume"

// DefaultNamespace constant for default namespace
const DefaultNamespace = "default"

// ServiceAppLabel label name for service app
const ServiceAppLabel = "app"

// K8SAppLabel label name for k8s app
const K8SAppLabel = "k8s-app"

// HyperOperatorModeLabel label name for hyper mode
const HyperOperatorModeLabel = "hyper-mode"

// in order to create a VMO one needs to provide a k8s secret with keys
// various secrets needed by vmo

// VMOSecretUsernameField constant for username
const VMOSecretUsernameField = "username"

// VMOSecretPasswordField constant for password
const VMOSecretPasswordField = "password"

// TLSCRTName constant for tls crt
const TLSCRTName = "tls.crt"

// TLSKeyName constant for tls key
const TLSKeyName = "tls.key"

// MetricsNameSpace constant for metrics namespace
const MetricsNameSpace = "vmo_operator"

// DefaultPrometheusRetentionPeriod default Prometheus retention configuration
const DefaultPrometheusRetentionPeriod = 90

// ESHttpPort default Elasticsearch HTTP port
const ESHttpPort = 9200

// ESTransportPort default Elasticsearch transport port
const ESTransportPort = 9300

// OidcProxyPort default OidcProxy HTTP port
const OidcProxyPort = 8775

// DefaultDevProfileESMemArgs default Elasticsearch dev mode memory settings
const DefaultDevProfileESMemArgs = "-Xms700m -Xmx700m"

// DefaultESIngestMemArgs default Elasticsearch Ingest memory settings
const DefaultESIngestMemArgs = "-Xms2g -Xmx2g"

// DefaultESDataMemArgs default Elasticsearch Data memory settings
const DefaultESDataMemArgs = "-Xms4g -Xmx4g"

// K8sTaintNoScheduleEffect constant for Noschedule
const K8sTaintNoScheduleEffect = "NoSchedule"

// K8sReadyCondition constant for Ready
const K8sReadyCondition = "Ready"

// K8sZoneLabel constant used for affinity
const K8sZoneLabel = "failure-domain.beta.kubernetes.io/zone"

// DashboardConfig dashboards config
const DashboardConfig = "dashboards"

// DatasourceConfig dashboard datasource config
const DatasourceConfig = "datasource"

// AlertrulesConfig canary alert rules config
const AlertrulesConfig = "alertrules"

// AlertrulesVersionsConfig canary alert rules config versions
const AlertrulesVersionsConfig = "alertrules-versions"

// AlertManagerYaml alertmanager yaml
const AlertManagerYaml = "alertmanager.yml"

// AlertManagerConfig alertmanager config
const AlertManagerConfig = "alertmanager-config"

// AlertManagerConfigVersions alertmanager config versions
const AlertManagerConfigVersions = "alertmanager-config-versions"

// AlertManagerConfigMountPath alertmanager config mountpath
const AlertManagerConfigMountPath = "/etc/alertmanager/config"

// AlertManagerWebhookURL alertmanager webhook URL
const AlertManagerWebhookURL = "http://localhost:9093/-/reload"

// AlertManagerConfigContainerLocation alertmanager config inside container
const AlertManagerConfigContainerLocation = "/etc/alertmanager/config/" + AlertManagerYaml

// PrometheusConfig Prometheus config
const PrometheusConfig = "prometheus-config"

// PrometheusConfigVersions Prometheus config versions
const PrometheusConfigVersions = "prometheus-config-versions"

// PrometheusConfigMountPath Prometheus config mountpath
const PrometheusConfigMountPath = "/etc/prometheus/config"

// IstioCertsMountPath Istio certs mountpath
const IstioCertsMountPath = "/etc/istio-certs"

// PrometheusRulesMountPath Prometheus rules mountpath
const PrometheusRulesMountPath = "/etc/prometheus/rules"

// PrometheusConfigContainerLocation Prometheus config inside container
const PrometheusConfigContainerLocation = "/etc/prometheus/config/prometheus.yml"

// ExternalDNSTTLSeconds value used for ingress annotation
const ExternalDNSTTLSeconds = 60

// NginxClientMaxBodySize value used for ingress annotation
const NginxClientMaxBodySize = "6M"

// NginxProxyReadTimeoutForKibana value used for ingress annotation
const NginxProxyReadTimeoutForKibana = "210s"

// DefaultElasticsearchDataReplicas default replicas for ESData
const DefaultElasticsearchDataReplicas = 1

// DefaultElasticsearchMasterReplicas default replicas for ESMaster
const DefaultElasticsearchMasterReplicas = 1

// DefaultElasticsearchIngestReplicas default replicas for ESIngest
const DefaultElasticsearchIngestReplicas = 1

// Storage-related constants

// OciFlexVolumeProvisioner flex volume provisioner for OCI
const OciFlexVolumeProvisioner = "oracle.com/oci"

// OciAvailabilityDomainLabel availability domain for OCI
const OciAvailabilityDomainLabel = "oci-availability-domain"

// K8sDefaultStorageClassAnnotation annotation for default storage class
const K8sDefaultStorageClassAnnotation = "storageclass.kubernetes.io/is-default-class"

// K8sDefaultStorageClassBetaAnnotation annotation for default storage class beta flavor
const K8sDefaultStorageClassBetaAnnotation = "storageclass.beta.kubernetes.io/is-default-class"

// MonitoringNamespace Monitoring namespace
const MonitoringNamespace = "monitoring"

// MCRegistrationSecret - the name of the secret that contains the cluster registration information
const MCRegistrationSecret = "verrazzano-cluster-registration"

// ClusterNameData - the field name in MCRegistrationSecret that contains this managed cluster's name
const ClusterNameData = "managed-cluster-name"

// KeycloakURLData - the field name in MCRegistrationSecret that contains the admin cluster's Keycloak endpoint's URL
const KeycloakURLData = "keycloak-url"

// KeycloakCABundleData - the field name in MCRegistrationSecret that contains the admin cluster's Keycloak ca-bundle
const KeycloakCABundleData = "ca-bundle"
