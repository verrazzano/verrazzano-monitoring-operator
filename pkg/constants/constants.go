// Copyright (C) 2020, Oracle and/or its affiliates.
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

// VMOLabel to be applied to all components of a VMO
const VMOGroup = "verrazzano.io"
const VMOVersion = "v1"
const VMOLabel = "vmo." + VMOVersion + "." + VMOGroup
const VMOKind = "VerrazzanoMonitoringInstance"
const VMOPlural = "verrazzanomonitoringinstances"
const VMOFullname = VMOPlural + "." + VMOGroup

// RoleBindingForVMOInstance rolebinding name for VMO instance
const RoleBindingForVMOInstance = "verrazzano-monitoring-operator"

// ClusterRoleForVMOInstances clusterrole name for VMO instance
const ClusterRoleForVMOInstances = "vmi-cluster-role"

// ResyncPeriod (re-list time period) for VMO Controller
const ResyncPeriod = 30 * time.Second

// VMOServiceNamePrefix to be applied to all VMO services
const VMOServiceNamePrefix = "vmi-"

const StorageVolumeName = "storage-volume"
const DefaultNamespace = "default"
const ServiceAppLabel = "app"
const K8SAppLabel = "k8s-app"

const HyperOperatorModeLabel = "hyper-mode"

// in order to create a VMO one needs to provide a k8s secret with keys
// various secrets needed by vmo
const VMOSecretUsernameField = "username"
const VMOSecretPasswordField = "password"

// TLS secrets
const TLSCRTName = "tls.crt"
const TLSKeyName = "tls.key"

//VMO Metrics
const MetricsNameSpace = "vmo_operator"

// Default Prometheus retention configuration
const DefaultPrometheusRetentionPeriod = 90

const ESHttpPort = 9200
const ESTransportPort = 9300
const DefaultESIngestMemArgs = "-Xms2g -Xmx2g"
const DefaultESDataMemArgs = "-Xms4g -Xmx4g"

// Various Kubernetes constants
const K8sTaintNoScheduleEffect = "NoSchedule"
const K8sReadyCondition = "Ready"
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

// AlertManagerWebhookURL alertmanager webhook url
const AlertManagerWebhookURL = "http://localhost:9093/-/reload"

// AlertManagerConfigContainerLocation alertmanager config inside container
const AlertManagerConfigContainerLocation = "/etc/alertmanager/config/" + AlertManagerYaml

// PrometheusConfig prometheus config
const PrometheusConfig = "prometheus-config"

// PrometheusConfigVersions prometheus config versions
const PrometheusConfigVersions = "prometheus-config-versions"

// PrometheusConfigMountPath prometheus config mountpath
const PrometheusConfigMountPath = "/etc/prometheus/config"

// PrometheusRulesMountPath prometheus rules mountpath
const PrometheusRulesMountPath = "/etc/prometheus/rules"

// PrometheusConfigContainerLocation prometheus config inside container
const PrometheusConfigContainerLocation = "/etc/prometheus/config/prometheus.yml"

// PrometheusNodeExporterPath prometheus node exporter mountpath
const PrometheusNodeExporterPath = "/prometheus-disk"

// ElasticSearchNodeExporterPath prometheus node exporter mountpath
const ElasticSearchNodeExporterPath = "/elasticsearch-disk"

// External DNS constants
const ExternalDnsTTLSeconds = 60

// External site monitor constants
const NginxClientMaxBodySize = "6M"
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
