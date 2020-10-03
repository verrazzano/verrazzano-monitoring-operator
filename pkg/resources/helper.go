// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package resources

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// GetMetaName returns name
func GetMetaName(vmoName string, componentName string) string {
	return constants.VMOServiceNamePrefix + vmoName + "-" + componentName
}

// GetMetaLabels returns k8s-app and vmo lables
func GetMetaLabels(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) map[string]string {
	return map[string]string{constants.K8SAppLabel: constants.VMOGroup, constants.VMOLabel: vmo.Name}
}

// GetSpecID returns app label
func GetSpecID(vmoName string, componentName string) map[string]string {
	return map[string]string{constants.ServiceAppLabel: vmoName + "-" + componentName}
}

// GetServicePort returns service port
func GetServicePort(componentDetails config.ComponentDetails) corev1.ServicePort {
	return corev1.ServicePort{Name: componentDetails.Name, Port: int32(componentDetails.Port)}
}

// GetOwnerReferences returns owner references
func GetOwnerReferences(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []metav1.OwnerReference {
	var ownerReferences []metav1.OwnerReference
	if vmo.Spec.CascadingDelete {
		ownerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(vmo, schema.GroupVersionKind{
				Group:   vmcontrollerv1.SchemeGroupVersion.Group,
				Version: vmcontrollerv1.SchemeGroupVersion.Version,
				Kind:    constants.VMOKind,
			}),
		}
	}
	return ownerReferences
}

// SliceContains returns whether or not the given slice contains the given string
func SliceContains(slice []string, value string) bool {
	for _, a := range slice {
		if a == value {
			return true
		}
	}
	return false
}

// GetStorageElementForComponent returns storage for a given component
func GetStorageElementForComponent(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) (storage *vmcontrollerv1.Storage) {
	switch component.Name {
	case config.Grafana.Name:
		return &vmo.Spec.Grafana.Storage
	case config.Prometheus.Name:
		return &vmo.Spec.Prometheus.Storage
	case config.ElasticsearchData.Name:
		return &vmo.Spec.Elasticsearch.Storage
	}
	return nil
}

// GetReplicasForComponent returns number of replicas for a given component
func GetReplicasForComponent(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) (replicas int32) {
	switch component.Name {
	case config.Grafana.Name:
		return int32(1)
	case config.Prometheus.Name:
		return vmo.Spec.Prometheus.Replicas
	case config.ElasticsearchData.Name:
		return vmo.Spec.Elasticsearch.DataNode.Replicas
	}
	return 0
}

// GetNextStringInSequence returns the next string in the incremental sequence given a string
func GetNextStringInSequence(name string) string {
	tokens := strings.Split(name, "-")
	if len(tokens) < 2 {
		return name + "-1" // Starting a new sequence
	}
	number, err := strconv.Atoi(tokens[len(tokens)-1])
	if err != nil {
		return name + "-1" // Starting a new sequence
	}
	tokens[len(tokens)-1] = strconv.Itoa(number + 1)
	return strings.Join(tokens, "-")
}

// CreateContainerElement creates a generic container element for the given component of the given VMO object.
func CreateContainerElement(vmoStorage *vmcontrollerv1.Storage,
	vmoResources *vmcontrollerv1.Resources, componentDetails config.ComponentDetails) corev1.Container {

	var volumeMounts []corev1.VolumeMount
	if vmoStorage != nil && vmoStorage.Size != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{MountPath: componentDetails.DataDir, Name: constants.StorageVolumeName})
	}

	limitResourceList := corev1.ResourceList{}
	requestResourceList := corev1.ResourceList{}
	if vmoResources != nil {
		if vmoResources.LimitCPU != "" {
			limitResourceList[corev1.ResourceCPU] = resource.MustParse(vmoResources.LimitCPU)
		}
		if vmoResources.LimitMemory != "" {
			limitResourceList[corev1.ResourceMemory] = resource.MustParse(vmoResources.LimitMemory)
		}
		if vmoResources.RequestCPU != "" {
			requestResourceList[corev1.ResourceCPU] = resource.MustParse(vmoResources.RequestCPU)
		}
		if vmoResources.RequestMemory != "" {
			requestResourceList[corev1.ResourceMemory] = resource.MustParse(vmoResources.RequestMemory)
		}
	}

	var livenessProbe *corev1.Probe = nil
	if componentDetails.LivenessHTTPPath != "" {
		livenessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   componentDetails.LivenessHTTPPath,
					Port:   intstr.IntOrString{IntVal: int32(componentDetails.Port)},
					Scheme: "HTTP",
				},
			},
		}
	}

	var readinessProbe *corev1.Probe = nil
	if componentDetails.ReadinessHTTPPath != "" {
		readinessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   componentDetails.ReadinessHTTPPath,
					Port:   intstr.IntOrString{IntVal: int32(componentDetails.Port)},
					Scheme: "HTTP",
				},
			},
		}
	}
	return corev1.Container{
		Name:            componentDetails.Name,
		Image:           componentDetails.Image,
		ImagePullPolicy: constants.DefaultImagePullPolicy,
		SecurityContext: &corev1.SecurityContext{
			Privileged: &componentDetails.Privileged,
		},
		Ports: []corev1.ContainerPort{{Name: componentDetails.Name, ContainerPort: int32(componentDetails.Port)}},
		Resources: corev1.ResourceRequirements{
			Requests: requestResourceList,
			Limits:   limitResourceList,
		},
		VolumeMounts:   volumeMounts,
		LivenessProbe:  livenessProbe,
		ReadinessProbe: readinessProbe,
	}
}

// CreateZoneAntiAffinityElement return an Affinity resource for a given VMO instance and component
func CreateZoneAntiAffinityElement(vmoName string, component string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: GetSpecID(vmoName, component),
						},
						TopologyKey: constants.K8sZoneLabel,
					},
				},
			},
		},
	}
}

// GetElasticsearchInitContainerMaxMapCount returns an Elasticsearch Init container that sets vm.max_map_count
func GetElasticsearchInitContainerMaxMapCount() *corev1.Container {
	elasticsearchInitContainer := CreateContainerElement(nil, nil, config.ElasticsearchInit)
	elasticsearchInitContainer.Command = []string{"sh", "-c"}
	elasticsearchInitContainer.Args = []string{"sysctl -w vm.max_map_count=262144",
		"chown -R 1000:1000 /usr/share/elasticsearch/data"}
	elasticsearchInitContainer.Ports = nil
	return &elasticsearchInitContainer
}

// GetElasticsearchInitContainerChown return an Elasticsearch Init container that changes owernsip of the ES directory.
// This is needed to access the PV volume data
func GetElasticsearchInitContainerChown() *corev1.Container {
	elasticsearchInitContainer := CreateContainerElement(nil, nil, config.ElasticsearchInitChown)
	elasticsearchInitContainer.Command = []string{"sh", "-c", "chown -R 1000:1000 /usr/share/elasticsearch/data"}
	elasticsearchInitContainer.Ports = nil
	return &elasticsearchInitContainer
}

// GetDefaultPrometheusConfiguration returns the default Prometheus configuration for a VMO instance
func GetDefaultPrometheusConfiguration(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) string {
	pushGWUrl := GetMetaName(vmo.Name, config.PrometheusGW.Name) + ":" + strconv.Itoa(config.PrometheusGW.Port)
	alertmanagerURL := fmt.Sprintf(GetMetaName(vmo.Name, config.AlertManager.Name)+":%d", config.AlertManager.Port)
	// Prometheus does not allow any special characters in their label names, So they need to be removed using reg exp
	re := regexp.MustCompile("[^a-zA-Z0-9_]")
	prometheusValidLabelName := re.ReplaceAllString(vmo.Name, "")
	dynamicScrapeAnnotation := prometheusValidLabelName + "_io_scrape"
	namespace := vmo.Namespace
	nginxNamespace := "ingress-nginx"
	var prometheusConfig = []byte(`
global:
  scrape_interval: 20s
  evaluation_interval: 30s
rule_files:
  - '/etc/prometheus/rules/*.rules'
alerting:
  alertmanagers:
    - static_configs:
      - targets: ["` + alertmanagerURL + `"]
scrape_configs:
 - job_name: 'prometheus'
   scrape_interval: 20s
   scrape_timeout: 15s
   static_configs:
   - targets: ['localhost:9090']
 - job_name: 'PushGateway'
   honor_labels: true
   scrape_interval: 20s
   scrape_timeout: 15s
   static_configs:
   - targets: ["` + pushGWUrl + `"]
 - job_name: 'cadvisor'
   scrape_interval: 20s
   scrape_timeout: 15s
   kubernetes_sd_configs:
   - role: node
   scheme: https
   tls_config:
     ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
     insecure_skip_verify: true
   bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
   relabel_configs:
   - action: labelmap
     regex: __meta_kubernetes_node_label_(.+)
   - target_label: __address__
     replacement: kubernetes.default.svc:443
   - source_labels: [__meta_kubernetes_node_name]
     regex: (.+)
     target_label: __metrics_path__
     replacement: /api/v1/nodes/$1/proxy/metrics/cadvisor
 - job_name: 'kubernetes-pods'
   kubernetes_sd_configs:
   - role: pod
     namespaces:
       names:
         - "` + namespace + `"
         - "` + nginxNamespace + `"
   relabel_configs:
   - source_labels: [__meta_kubernetes_pod_annotation_` + dynamicScrapeAnnotation + `]
     action: keep
     regex: true
   - action: labelmap
     regex: __meta_kubernetes_pod_label_(.+)
   - source_labels: [__meta_kubernetes_namespace]
     action: replace
     target_label: kubernetes_namespace
   - source_labels: [__meta_kubernetes_pod_name]
     action: replace
     target_label: kubernetes_pod_name`)

	return string(prometheusConfig)
}

// GetDefaultAlertManagerConfiguration returns the default Prometheus configuration for a VMO instance
func GetDefaultAlertManagerConfiguration(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) string {

	name := vmo.Name

	var alertManagerConfig = []byte(`
route:
  receiver: "` + name + `"
  group_by: ['alertname']
  group_wait: 30s
  group_interval: 1m
  repeat_interval: 3m
receivers:
- name: "` + name + `"
  pagerduty_configs:
  - service_key: changeme`)

	return string(alertManagerConfig)
}

// NewVal return a pointer to an int32 given an int32 value
func NewVal(value int32) *int32 {
	var val = value
	return &val
}

// New64Val return a pointer to an int64 given an int64 value
func New64Val(value int64) *int64 {
	var val = value
	return &val
}

// NewBool return a pointer to a boolean given a boolean value
func NewBool(value bool) *bool {
	var val = value
	return &val
}
