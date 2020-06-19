// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
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

func GetMetaName(sauronName string, componentName string) string {
	return constants.SauronServiceNamePrefix + sauronName + "-" + componentName
}

func GetMetaLabels(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) map[string]string {
	return map[string]string{constants.K8SAppLabel: constants.SauronGroup, constants.SauronLabel: sauron.Name}
}

func GetSpecId(sauronName string, componentName string) map[string]string {
	return map[string]string{constants.ServiceAppLabel: sauronName + "-" + componentName}
}

func GetServicePort(componentDetails config.ComponentDetails) corev1.ServicePort {
	return corev1.ServicePort{Name: componentDetails.Name, Port: int32(componentDetails.Port)}
}

func GetOwnerReferences(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) []metav1.OwnerReference {
	var ownerReferences []metav1.OwnerReference
	if sauron.Spec.CascadingDelete {
		ownerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(sauron, schema.GroupVersionKind{
				Group:   vmcontrollerv1.SchemeGroupVersion.Group,
				Version: vmcontrollerv1.SchemeGroupVersion.Version,
				Kind:    constants.SauronKind,
			}),
		}
	}
	return ownerReferences
}

// Returns whether or not the given slice contains the given string
func SliceContains(slice []string, value string) bool {
	for _, a := range slice {
		if a == value {
			return true
		}
	}
	return false
}

func GetStorageElementForComponent(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) (storage *vmcontrollerv1.Storage) {
	switch component.Name {
	case config.Grafana.Name:
		return &sauron.Spec.Grafana.Storage
	case config.Prometheus.Name:
		return &sauron.Spec.Prometheus.Storage
	case config.ElasticsearchData.Name:
		return &sauron.Spec.Elasticsearch.Storage
	}
	return nil
}

// Returns number of replicas for a given component
func GetReplicasForComponent(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, component *config.ComponentDetails) (replicas int32) {
	switch component.Name {
	case config.Grafana.Name:
		return int32(1)
	case config.Prometheus.Name:
		return sauron.Spec.Prometheus.Replicas
	case config.ElasticsearchData.Name:
		return sauron.Spec.Elasticsearch.DataNode.Replicas
	}
	return 0
}

// Given a string, returns the next string in the incremental sequence.
func GetNextStringInSequence(name string) string {
	tokens := strings.Split(name, "-")
	if len(tokens) < 2 {
		return name + "-1" // Starting a new sequence
	} else {
		number, err := strconv.Atoi(tokens[len(tokens)-1])
		if err != nil {
			return name + "-1" // Starting a new sequence
		} else {
			tokens[len(tokens)-1] = strconv.Itoa(number + 1)
			return strings.Join(tokens, "-")
		}
	}
}

// Creates a generic container element for the given component of the given Sauron object.
func CreateContainerElement(sauronStorage *vmcontrollerv1.Storage,
	sauronResources *vmcontrollerv1.Resources, componentDetails config.ComponentDetails) corev1.Container {

	var volumeMounts []corev1.VolumeMount
	if sauronStorage != nil && sauronStorage.Size != "" {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{MountPath: componentDetails.DataDir, Name: constants.StorageVolumeName})
	}

	limitResourceList := corev1.ResourceList{}
	requestResourceList := corev1.ResourceList{}
	if sauronResources != nil {
		if sauronResources.LimitCPU != "" {
			limitResourceList[corev1.ResourceCPU] = resource.MustParse(sauronResources.LimitCPU)
		}
		if sauronResources.LimitMemory != "" {
			limitResourceList[corev1.ResourceMemory] = resource.MustParse(sauronResources.LimitMemory)
		}
		if sauronResources.RequestCPU != "" {
			requestResourceList[corev1.ResourceCPU] = resource.MustParse(sauronResources.RequestCPU)
		}
		if sauronResources.RequestMemory != "" {
			requestResourceList[corev1.ResourceMemory] = resource.MustParse(sauronResources.RequestMemory)
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

func CreateZoneAntiAffinityElement(sauronName string, component string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: GetSpecId(sauronName, component),
						},
						TopologyKey: constants.K8sZoneLabel,
					},
				},
			},
		},
	}
}

// Returns an Elasticsearch Init container object
func GetElasticsearchInitContainer() *corev1.Container {
	elasticsearchInitContainer := CreateContainerElement(nil, nil, config.ElasticsearchInit)
	elasticsearchInitContainer.Args = []string{"sysctl", "-w", "vm.max_map_count=262144"}
	elasticsearchInitContainer.Ports = nil
	return &elasticsearchInitContainer
}

// Gets the default Prometheus configuration for a Sauron instance
func GetDefaultPrometheusConfiguration(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) string {
	pushGWUrl := GetMetaName(sauron.Name, config.PrometheusGW.Name) + ":" + strconv.Itoa(config.PrometheusGW.Port)
	alertmanagerURL := fmt.Sprintf(GetMetaName(sauron.Name, config.AlertManager.Name)+":%d", config.AlertManager.Port)
	// Prometheus does not allow any special characters in their label names, So they need to be removed using reg exp
	re := regexp.MustCompile("[^a-zA-Z0-9_]")
	prometheusValidLabelName := re.ReplaceAllString(sauron.Name, "")
	dynamicScrapeAnnotation := prometheusValidLabelName + "_io_scrape"
	namespace := sauron.Namespace
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

// Gets the default Prometheus configuration for a Sauron instance
func GetDefaultAlertManagerConfiguration(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance) string {

	name := sauron.Name

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

func NewVal(value int32) *int32 {
	var val = value
	return &val
}

func New64Val(value int64) *int64 {
	var val = value
	return &val
}

func NewBool(value bool) *bool {
	var val = value
	return &val
}
