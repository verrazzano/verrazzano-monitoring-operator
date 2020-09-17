// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package constants

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

const DefaultImagePullPolicy = corev1.PullIfNotPresent

const NobodyUID = 65534

type VMOStatus string

const (
	Running = VMOStatus("Running")
)

type StorageOperationType string

// VMOLabel to be applied to all components of a VMO
const VMOGroup = "verrazzano.io"
const VMOVersion = "v1"
const VMOLabel = "vmo." + VMOVersion + "." + VMOGroup
const VMOKind = "VerrazzanoMonitoringInstance"
const VMOPlural = "verrazzanomonitoringinstances"
const VMOFullname = VMOPlural + "." + VMOGroup

const RoleBindingForVMOInstance = "verrazzano-monitoring-operator"
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

//dashboards config
const DashboardConfig = "dashboards"

//dashboard datasource config
const DatasourceConfig = "datasource"

//canary alert rules config
const AlertrulesConfig = "alertrules"

//canary alert rules config versions
const AlertrulesVersionsConfig = "alertrules-versions"

//alertmanager yaml
const AlertManagerYaml = "alertmanager.yml"

//alertmanager config
const AlertManagerConfig = "alertmanager-config"

//alertmanager config versions
const AlertManagerConfigVersions = "alertmanager-config-versions"

//alertmanager config mountpath
const AlertManagerConfigMountPath = "/etc/alertmanager/config"

//alertmanager webhook url
const AlertManagerWebhookURL = "http://localhost:9093/-/reload"

//alertmanager config inside container
const AlertManagerConfigContainerLocation = "/etc/alertmanager/config/" + AlertManagerYaml

//prometheus config
const PrometheusConfig = "prometheus-config"

//prometheus config versions
const PrometheusConfigVersions = "prometheus-config-versions"

//prometheus config mountpath
const PrometheusConfigMountPath = "/etc/prometheus/config"

//prometheus rules mountpath
const PrometheusRulesMountPath = "/etc/prometheus/rules"

//prometheus config inside container
const PrometheusConfigContainerLocation = "/etc/prometheus/config/prometheus.yml"

//prometheus node exporter mountpath
const PrometheusNodeExporterPath = "/prometheus-disk"

//prometheus node exporter mountpath
const ElasticSearchNodeExporterPath = "/elasticsearch-disk"

// External DNS constants
const ExternalDnsTTLSeconds = 60

// External site monitor constants
const NginxClientMaxBodySize = "6M"
const NginxProxyReadTimeoutForKibana = "210s"

const DefaultElasticsearchDataReplicas = 1
const DefaultElasticsearchMasterReplicas = 1
const DefaultElasticsearchIngestReplicas = 1

// Storage-related constants
const OciFlexVolumeProvisioner = "oracle.com/oci"
const OciAvailabilityDomainLabel = "oci-availability-domain"
const K8sDefaultStorageClassAnnotation = "storageclass.kubernetes.io/is-default-class"
const K8sDefaultStorageClassBetaAnnotation = "storageclass.beta.kubernetes.io/is-default-class"

// Monitoring namespace
const MonitoringNamespace = "monitoring"
