// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package v1

import (
	"encoding/json"
	"hash/fnv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

		// Elasticsearch details
		Elasticsearch Elasticsearch `json:"elasticsearch"`

		// Kibana details
		Kibana Kibana `json:"kibana"`

		// API details
		API API `json:"api,omitempty"`

		// Service type for component services
		ServiceType corev1.ServiceType `json:"serviceType" yaml:"serviceType"`

		ContactEmail string `json:"contactemail,omitempty" yaml:"contactemail,omitempty"`

		NatGatewayIPs []string `json:"natGatewayIPs,omitempty" yaml:"natGatewayIPs,omitempty"`
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
		HTTP2Enabled           bool      `json:"http2Enabled" yaml:"http2Enabled"`
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

	// Elasticsearch details
	Elasticsearch struct {
		Enabled    bool              `json:"enabled" yaml:"enabled"`
		Storage    Storage           `json:"storage,omitempty"`
		IngestNode ElasticsearchNode `json:"ingestNode,omitempty"`
		MasterNode ElasticsearchNode `json:"masterNode,omitempty"`
		DataNode   ElasticsearchNode `json:"dataNode,omitempty"`
	}

	// ElasticsearchNode Type details
	ElasticsearchNode struct {
		Replicas  int32     `json:"replicas,omitempty"`
		JavaOpts  string    `json:"javaOpts" yaml:"javaOpts,omitempty"`
		Resources Resources `json:"resources,omitempty"`
	}

	// Kibana details
	Kibana struct {
		Enabled   bool      `json:"enabled" yaml:"enabled"`
		Resources Resources `json:"resources,omitempty"`
		Replicas  int32     `json:"replicas,omitempty"`
	}

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
		Hash         uint32       `json:"hash"`
	}

	// Storage details
	Storage struct {
		Size               string   `json:"size,omitempty" yaml:"size"`
		AvailabilityDomain string   `json:"availabilityDomain,omitempty" yaml:"availabilityDomain"`
		PvcNames           []string `json:"pvcNames,omitempty" yaml:"pvcNames,omitempty"`
	}

	// Resources details
	Resources struct {
		LimitCPU      string `json:"limitCPU,omitempty"`
		LimitMemory   string `json:"limitMemory,omitempty"`
		RequestCPU    string `json:"requestCPU,omitempty"`
		RequestMemory string `json:"requestMemory,omitempty"`
		MaxSizeDisk   string `json:"maxSizeDisk,omitempty" yaml:"maxSizeDisk,omitempty"`
		MinSizeDisk   string `json:"minSizeDisk,omitempty" yaml:"minSizeDisk,omitempty"`
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
	VerrazzanoMonitoringInstance struct {
		metav1.TypeMeta   `json:",inline"`
		metav1.ObjectMeta `json:"metadata"`
		Spec              VerrazzanoMonitoringInstanceSpec   `json:"spec"`
		Status            VerrazzanoMonitoringInstanceStatus `json:"status"`
	}

	// VerrazzanoMonitoringInstanceList Represents a collection of CRDs
	// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
	VerrazzanoMonitoringInstanceList struct {
		metav1.TypeMeta `json:",inline"`
		metav1.ListMeta `json:"metadata"`
		Items           []VerrazzanoMonitoringInstance `json:"items"`
	}
)

// Hash function to identify VerrazzanoMonitoringInstance changes
func (c *VerrazzanoMonitoringInstance) Hash() (uint32, error) {
	b, err := json.Marshal(c.Spec)
	if err != nil {
		return 0, err
	}
	h := fnv.New32a()
	if _, err := h.Write(b); err != nil {
		return 0, err
	}
	return h.Sum32(), nil
}

// GetObjectKind to get kind
func (c *VerrazzanoMonitoringInstance) GetObjectKind() schema.ObjectKind {
	return &c.TypeMeta
}

// GetObjectKind to get kind
func (c *VerrazzanoMonitoringInstanceList) GetObjectKind() schema.ObjectKind {
	return &c.TypeMeta
}
