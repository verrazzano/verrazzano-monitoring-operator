// Copyright (C) 2020, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type NodeRole string

const (
	MasterRole NodeRole = "master"
	DataRole   NodeRole = "data"
	IngestRole NodeRole = "ingest"
	// OpportunisticStartTLS means that SMTP transactions are encrypted if STARTTLS is supported by the SMTP server.
	// Otherwise, messages are sent in the clear.
	OpportunisticStartTLS StartTLSType = "OpportunisticStartTLS"
	// MandatoryStartTLS means that SMTP transactions must be encrypted.
	// SMTP transactions are aborted unless STARTTLS is supported by the SMTP server.
	MandatoryStartTLS StartTLSType = "MandatoryStartTLS"
	// NoStartTLS means encryption is disabled and messages are sent in the clear.
	NoStartTLS StartTLSType = "NoStartTLS"
)

type (

	// VerrazzanoMonitoringInstanceSpec defines the attributes a user can specify when creating a VerrazzanoMonitoringInstance
	VerrazzanoMonitoringInstanceSpec struct {

		// Version details
		Versioning Versioning `json:"versioning,omitempty" yaml:"versioning"`

		// If lock, controller will not sync/process the VerrazzanoMonitoringInstance env
		Lock bool `json:"lock" yaml:"lock"`

		// the external endpoint or uniform resource identifier
		URI string `json:"uri,omitempty" yaml:"uri"`

		// the ingress class for external endpoints
		IngressClassName *string `json:"IngressClassName,omitempty"`

		// a secret which contains secrets VerrazzanoMonitoringInstance needs to startup
		// examples being username, password, tls.crt, tls.key
		SecretsName string `json:"secretsName" yaml:"secretsName"`

		// the nginx ingress controller tls cert secret
		SecretName string `json:"-" yaml:"-"`

		// auto generate a SSL certificate
		AutoSecret bool `json:"autoSecret" yaml:"autoSecret"`

		// Will use this as the target in ingress annotations, use this when using OCI LB and
		// external-dns so that we point to the svc CNAME created
		IngressTargetDNSName string `json:"ingressTargetDNSName" yaml:"ingressTargetDNSName"`

		// CascadingDelete for cascade deletion of related objects when the VerrazzanoMonitoringInstance is deleted
		CascadingDelete bool `json:"cascadingDelete" yaml:"cascadingDelete"`

		// Grafana details
		Grafana Grafana `json:"grafana"`

		// Prometheus details
		Prometheus Prometheus `json:"prometheus"`

		// Prometheus details
		AlertManager AlertManager `json:"alertmanager"`

		// Deprecated: Elasticsearch has been replaced by OpenSearch
		// +optional
		Elasticsearch *Elasticsearch `json:"elasticsearch,omitempty"`

		// OpenSearch details
		Opensearch Opensearch `json:"opensearch"`

		// Deprecated: Kibana has been replaced by OpenSearch Dashboards
		// +optional
		Kibana *Kibana `json:"kibana,omitempty"`

		// OpenSearch Dashboards details
		OpensearchDashboards OpensearchDashboards `json:"opensearchDashboards"`

		// API details
		API API `json:"api,omitempty"`

		// Service type for component services
		ServiceType corev1.ServiceType `json:"serviceType" yaml:"serviceType"`

		ContactEmail string `json:"contactemail,omitempty" yaml:"contactemail,omitempty"`

		NatGatewayIPs []string `json:"natGatewayIPs,omitempty" yaml:"natGatewayIPs,omitempty"`

		// +optional
		StorageClass *string `json:"storageClass,omitempty"`
	}

	// Versioning details
	Versioning struct {
		CurrentVersion string `json:"currentVersion,omitempty" yaml:"currentVersion"`
		DesiredVersion string `json:"desiredVersion,omitempty" yaml:"desiredVersion"`
	}

	// Grafana details
	Grafana struct {
		Enabled              bool      `json:"enabled" yaml:"enabled"`
		Storage              Storage   `json:"storage,omitempty"`
		DatasourcesConfigMap string    `json:"datasourcesConfigMap,omitempty"`
		DashboardsConfigMap  string    `json:"dashboardsConfigMap,omitempty"`
		Resources            Resources `json:"resources,omitempty"`
		Replicas             int32     `json:"replicas,omitempty"`
		Database             *Database `json:"database,omitempty"`
		SMTP                 *SMTPInfo `json:"smtp,omitempty"`
	}

	// Prometheus details
	Prometheus struct {
		Enabled                bool      `json:"enabled" yaml:"enabled"`
		Storage                Storage   `json:"storage,omitempty"`
		ConfigMap              string    `json:"configMap,omitempty"`
		VersionsConfigMap      string    `json:"versionsConfigMap,omitempty"`
		RulesConfigMap         string    `json:"rulesConfigMap,omitempty"`
		RulesVersionsConfigMap string    `json:"rulesVersionsConfigMap,omitempty"`
		Resources              Resources `json:"resources,omitempty"`
		RetentionPeriod        int32     `json:"retentionPeriod,omitempty"`
		Replicas               int32     `json:"replicas,omitempty"`
		HTTP2Enabled           bool      `json:"http2Enabled,omitempty" yaml:"http2Enabled"`
	}

	// AlertManager details
	AlertManager struct {
		Enabled           bool      `json:"enabled" yaml:"enabled"`
		Config            string    `json:"config,omitempty"`
		ConfigMap         string    `json:"configMap,omitempty"`
		VersionsConfigMap string    `json:"versionsConfigMap,omitempty"`
		Resources         Resources `json:"resources,omitempty"`
		Replicas          int32     `json:"replicas,omitempty"`
	}

	// Deprecated: Elasticsearch type has been replaced by Opensearch
	Elasticsearch struct {
		Enabled              bool                    `json:"enabled" yaml:"enabled"`
		Storage              Storage                 `json:"storage,omitempty"`
		IngestNode           ElasticsearchNode       `json:"ingestNode,omitempty"`
		MasterNode           ElasticsearchNode       `json:"masterNode,omitempty"`
		DataNode             ElasticsearchNode       `json:"dataNode,omitempty"`
		Policies             []IndexManagementPolicy `json:"policies,omitempty"`
		Nodes                []ElasticsearchNode     `json:"nodes,omitempty"`
		Plugins              OpenSearchPlugins       `json:"plugins,omitempty"`
		DisableDefaultPolicy bool                    `json:"disableDefaultPolicy,omitempty"`
	}

	// Opensearch details
	Opensearch struct {
		Enabled              bool                    `json:"enabled" yaml:"enabled"`
		Storage              Storage                 `json:"storage,omitempty"`
		IngestNode           ElasticsearchNode       `json:"ingestNode,omitempty"`
		MasterNode           ElasticsearchNode       `json:"masterNode,omitempty"`
		DataNode             ElasticsearchNode       `json:"dataNode,omitempty"`
		Policies             []IndexManagementPolicy `json:"policies,omitempty"`
		Nodes                []ElasticsearchNode     `json:"nodes,omitempty"`
		Plugins              OpenSearchPlugins       `json:"plugins,omitempty"`
		DisableDefaultPolicy bool                    `json:"disableDefaultPolicy,omitempty"`
	}

	// ElasticsearchNode Type details
	ElasticsearchNode struct {
		Name      string     `json:"name,omitempty"`
		Replicas  int32      `json:"replicas,omitempty"`
		JavaOpts  string     `json:"javaOpts" yaml:"javaOpts,omitempty"`
		Resources Resources  `json:"resources,omitempty"`
		Storage   *Storage   `json:"storage,omitempty"`
		Roles     []NodeRole `json:"roles,omitempty"`
	}

	//IndexManagementPolicy Defines a policy for managing indices
	IndexManagementPolicy struct {
		// Name of the policy
		PolicyName string `json:"policyName"`
		// Index pattern the policy will be matched to
		IndexPattern string `json:"indexPattern"`
		// Minimum age of an index before it is automatically deleted
		// +kubebuilder:validation:Pattern:=^[0-9]+(d|h|m|s|ms|micros|nanos)$
		MinIndexAge *string        `json:"minIndexAge,omitempty"`
		Rollover    RolloverPolicy `json:"rollover,omitempty"`
	}

	//RolloverPolicy Settings for Index Management rollover
	RolloverPolicy struct {
		// Minimum age of an index before it is rolled over
		// +kubebuilder:validation:Pattern:=^[0-9]+(d|h|m|s|ms|micros|nanos)$
		MinIndexAge *string `json:"minIndexAge,omitempty"`
		// Minimum size of an index before it is rolled over
		// e.g., 20mb, 5gb, etc.
		// +kubebuilder:validation:Pattern:=^[0-9]+(b|kb|mb|gb|tb|pb)$
		MinSize *string `json:"minSize,omitempty"`
		// Minimum count of documents in an index before it is rolled over
		MinDocCount *int `json:"minDocCount,omitempty"`
	}

	// Deprecated: Kibana type has been replaced by OpensearchDashboards
	Kibana struct {
		Enabled   bool                        `json:"enabled" yaml:"enabled"`
		Resources Resources                   `json:"resources,omitempty"`
		Replicas  int32                       `json:"replicas,omitempty"`
		Plugins   OpenSearchDashboardsPlugins `json:"plugins,omitempty"`
	}

	// OpenSearch Dashboards details
	OpensearchDashboards struct {
		Enabled   bool                        `json:"enabled" yaml:"enabled"`
		Resources Resources                   `json:"resources,omitempty"`
		Replicas  int32                       `json:"replicas,omitempty"`
		Plugins   OpenSearchDashboardsPlugins `json:"plugins,omitempty"`
	}

	// OpenSearchPlugins Enable to add 3rd Party / Custom plugins not offered in the default OpenSearch image
	OpenSearchPlugins struct {
		// To enable or disable the non-bundled plugins installation.
		Enabled bool `json:"enabled" yaml:"enabled"`
		// InstallList could be the list of plugin names, URLs to the plugin zip file or Maven coordinates.
		InstallList []string `json:"installList,omitempty"`
	}

	// OpenSearchDashboardsPlugins is an alias of OpenSearchPlugins as both have the same properties.
	// Enable to add 3rd Party / Custom plugins not offered in the default OpenSearch-Dashboards image
	OpenSearchDashboardsPlugins OpenSearchPlugins

	// API details
	API struct {
		Replicas int32 `json:"replicas,omitempty"`
	}

	// VerrazzanoMonitoringInstanceStatus Object tracks the current running VerrazzanoMonitoringInstance state
	VerrazzanoMonitoringInstanceStatus struct {
		// The name of the operator environment in which this VerrazzanoMonitoringInstance instance lives
		EnvName      string       `json:"envName" yaml:"envName"`
		State        string       `json:"state" yaml:"state"`
		CreationTime *metav1.Time `json:"creationTime,omitempty" yaml:"creationTime"`
	}

	// Storage details
	Storage struct {
		Size               string   `json:"size,omitempty" yaml:"size"`
		AvailabilityDomain string   `json:"availabilityDomain,omitempty" yaml:"availabilityDomain"`
		PvcNames           []string `json:"pvcNames,omitempty" yaml:"pvcNames,omitempty"`
	}

	// Resources details
	Resources struct {
		// +kubebuilder:validation:Pattern:=^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$
		LimitCPU string `json:"limitCPU,omitempty"`
		// +kubebuilder:validation:Pattern:=^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$
		LimitMemory string `json:"limitMemory,omitempty"`
		// +kubebuilder:validation:Pattern:=^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$
		RequestCPU string `json:"requestCPU,omitempty"`
		// +kubebuilder:validation:Pattern:=^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$
		RequestMemory string `json:"requestMemory,omitempty"`

		// These fields are not used anywhere
		// +kubebuilder:validation:Pattern:=^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$
		MaxSizeDisk string `json:"maxSizeDisk,omitempty" yaml:"maxSizeDisk,omitempty"`
		// +kubebuilder:validation:Pattern:=^([+-]?[0-9.]+)([eEinumkKMGTP]*[-+]?[0-9]*)$
		MinSizeDisk string `json:"minSizeDisk,omitempty" yaml:"minSizeDisk,omitempty"`
	}

	// Database details
	Database struct {
		PasswordSecret string `json:"passwordSecret,omitempty"`
		Host           string `json:"host,omitempty"`
		Name           string `json:"name,omitempty"`
	}

	// ContainerSpec represents a container image that needs to be run periodically
	ContainerSpec struct {
		Image            string                        `json:"image,omitempty"`
		ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
		Args             []string                      `json:"args,omitempty"`
		EnvFrom          []corev1.EnvFromSource        `json:"envFrom,omitempty" protobuf:"bytes,19,rep,name=envFrom"`
		Env              []corev1.EnvVar               `json:"env,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,7,rep,name=env"`
		Volumes          []corev1.Volume               `json:"volumes,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,1,rep,name=volumes"`
		VolumeMounts     []corev1.VolumeMount          `json:"volumeMounts,omitempty" patchStrategy:"merge" patchMergeKey:"mountPath" protobuf:"bytes,9,rep,name=volumeMounts"`
	}

	// ScriptSpec represents a script that needs to be run periodically
	ScriptSpec struct {
		Content string `json:"content,omitempty"`
	}

	// HTTPSpec represetns a script that needs to be run periodically
	HTTPSpec struct {
		Method    string
		TLSConfig string
	}

	// Metric that represents
	Metric struct {
		Name string `json:"name" yaml:"name"`

		// Metric Type. One of Gauge | Counter | Summary | Histogram
		Type string `json:"type" yaml:"type"`

		// Help Text message
		Help string `json:"help" yaml:"help"`

		// Labels is an Acceptable list that can be attached to a metric
		Labels []string `json:"labels" yaml:"labels"`

		//Percentiles to be used for Summary type metric
		Percentiles []float64 `json:"percentiles,omitempty" yaml:"percentiles"`

		//Buckets to be used for Histogram type metric
		Buckets []float64 `json:"buckets,omitempty" yaml:"buckets"`
	}

	// ContainerConfig describes config needed run a container
	ContainerConfig struct {
		Image   string
		Args    []string
		EnvFrom []corev1.EnvFromSource
		Env     []corev1.EnvVar
	}

	// ScriptConfig describes the config needed to run the script
	ScriptConfig struct {
		File string
	}

	// VerrazzanoMonitoringInstance Represents a CRD
	// +genclient
	// +genclient:noStatus
	// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
	// +kubebuilder:resource:shortName=vmi
	VerrazzanoMonitoringInstance struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata"`
		Spec              VerrazzanoMonitoringInstanceSpec   `json:"spec"`
		Status            VerrazzanoMonitoringInstanceStatus `json:"status,omitempty"`
	}

	// VerrazzanoMonitoringInstanceList Represents a collection of CRDs
	// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
	VerrazzanoMonitoringInstanceList struct {
		metav1.TypeMeta `json:",inline"`
		metav1.ListMeta `json:"metadata"`
		Items           []VerrazzanoMonitoringInstance `json:"items"`
	}

	// StartTLSType is the type of protocol command used to inform the email server that the email client wants to upgrade from
	// an insecure connection to a secure one using TLS or SSL.
	StartTLSType string

	// SMTPInfo specifies the SMTP connection information for the Grafana SMTP notifications.
	SMTPInfo struct {
		// If true, then the SMTP notifications will be enabled.
		// +optional
		Enabled *bool `json:"enabled,omitempty"`
		// The address:port connection information for the SMTP server.
		// +optional
		Host string `json:"host,omitempty"`
		// The name of an existing secret containing the SMTP credentials (username, password, certificate and key for accessing the SMTP server).
		// +optional
		ExistingSecret string `json:"existingSecret,omitempty"`
		// The key in the existing SMTP secret containing the username.
		// +optional
		UserKey string `json:"userKey,omitempty"`
		// The key in the existing SMTP secret containing the password.
		// +optional
		PasswordKey string `json:"passwordKey,omitempty"`
		// The key in the existing SMTP secret containing the certificate.
		// +optional
		CertFileKey string `json:"certFileKey,omitempty"`
		// The key in the existing SMTP secret containing the key for the certificate.
		// +optional
		KeyFileKey string `json:"keyFileKey,omitempty"`
		// If true, do not Verify SSL for SMTP server.
		// +optional
		SkipVerify *bool `json:"skipVerify,omitempty"`
		// Address used when sending out emails, default is admin@grafana.localhost.
		// +optional
		FromAddress string `json:"fromAddress,omitempty"`
		// Name to be used when sending out emails, default is Grafana.
		// +optional
		FromName string `json:"fromName,omitempty"`
		// Name to be used as client identity for EHLO(Extended HELO) in SMTP dialog, default is <instance_name>.
		// +optional
		EHLOIdentity string `json:"ehloIdentity,omitempty"`
		// Either “OpportunisticStartTLS”, “MandatoryStartTLS”, “NoStartTLS”. Default is empty.
		// +optional
		StartTLSPolicy StartTLSType `json:"startTLSPolicy,omitempty"`
	}
)

// GetObjectKind to get kind
func (c *VerrazzanoMonitoringInstance) GetObjectKind() schema.ObjectKind {
	return &c.TypeMeta
}

// GetObjectKind to get kind
func (c *VerrazzanoMonitoringInstanceList) GetObjectKind() schema.ObjectKind {
	return &c.TypeMeta
}
