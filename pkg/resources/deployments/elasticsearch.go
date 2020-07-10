// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package deployments

import (
	"fmt"
	"strings"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ElasticsearchBasic struct {
}

// Returns a common base deployment structure for all Elasticsearch components
func (es ElasticsearchBasic) createElasticsearchCommonDeployment(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, vmiStorage *vmcontrollerv1.Storage,
	vmiResources *vmcontrollerv1.Resources, componentDetails config.ComponentDetails, index int) *appsv1.Deployment {

	deploymentElement := createDeploymentElementByPvcIndex(vmi, vmiStorage, vmiResources, componentDetails, index)

	deploymentElement.Spec.Template.Spec.Containers[0].Env = append(deploymentElement.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		corev1.EnvVar{
			Name: "node.name",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		corev1.EnvVar{Name: "cluster.name", Value: vmi.Name})

	deploymentElement.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
		{Name: "http", ContainerPort: int32(constants.ESHttpPort)},
		{Name: "transport", ContainerPort: int32(constants.ESTransportPort)},
	}

	// Common Elasticsearch readiness and liveness settings
	if deploymentElement.Spec.Template.Spec.Containers[0].LivenessProbe != nil {
		deploymentElement.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = 60
		deploymentElement.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds = 3
		deploymentElement.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds = 20
		deploymentElement.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold = 5
	}
	if deploymentElement.Spec.Template.Spec.Containers[0].ReadinessProbe != nil {
		deploymentElement.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds = 60
		deploymentElement.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds = 3
		deploymentElement.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds = 10
		deploymentElement.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold = 10
	}

	// Add init containers
	deploymentElement.Spec.Template.Spec.InitContainers = append(deploymentElement.Spec.Template.Spec.InitContainers, *resources.GetElasticsearchInitContainer())

	// Istio does not work with ElasticSearch.  Uncomment the following line when istio is present
	// deploymentElement.Spec.Template.Annotations = map[string]string{"sidecar.istio.io/inject": "false"}

	var elasticsearchUID int64 = 1000
	deploymentElement.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = &elasticsearchUID
	return deploymentElement
}

// Creates all Elasticsearch Client deployment elements
func (es ElasticsearchBasic) createElasticsearchIngestDeploymentElements(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) []*appsv1.Deployment {
	javaOpts := constants.DefaultESIngestMemArgs
	if vmi.Spec.Elasticsearch.IngestNode.JavaOpts != "" {
		javaOpts = vmi.Spec.Elasticsearch.IngestNode.JavaOpts
	}

	elasticsearchIngestDeployment := es.createElasticsearchCommonDeployment(vmi, nil, &vmi.Spec.Elasticsearch.IngestNode.Resources, config.ElasticsearchIngest, -1)

	elasticsearchIngestDeployment.Spec.Replicas = resources.NewVal(vmi.Spec.Elasticsearch.IngestNode.Replicas)

	// Anti-affinity on other client zones
	elasticsearchIngestDeployment.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmi.Name, config.ElasticsearchIngest.Name)

	initialMasterNodes := make([]string, 0)
	masterReplicas := resources.NewVal(vmi.Spec.Elasticsearch.MasterNode.Replicas)
	var i int32
	for i = 0; i < *masterReplicas; i++ {
		initialMasterNodes = append(initialMasterNodes, resources.GetMetaName(vmi.Name, config.ElasticsearchMaster.Name)+"-"+fmt.Sprintf("%d", i))
	}
	elasticsearchIngestDeployment.Spec.Template.Spec.Containers[0].Env = append(elasticsearchIngestDeployment.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "discovery.seed_hosts", Value: resources.GetMetaName(vmi.Name, config.ElasticsearchMaster.Name)},
		corev1.EnvVar{Name: "cluster.initial_master_nodes", Value: strings.Join(initialMasterNodes, ",")},
		corev1.EnvVar{Name: "node.master", Value: "false"},
		corev1.EnvVar{Name: "NETWORK_HOST", Value: "0.0.0.0"},
		corev1.EnvVar{Name: "node.ingest", Value: "true"},
		corev1.EnvVar{Name: "node.data", Value: "false"},
		corev1.EnvVar{Name: "ES_JAVA_OPTS", Value: javaOpts},
	)

	return []*appsv1.Deployment{elasticsearchIngestDeployment}
}

// Creates all Elasticsearch Data deployment elements
func (es ElasticsearchBasic) createElasticsearchDataDeploymentElements(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment {
	javaOpts := constants.DefaultESDataMemArgs
	if vmi.Spec.Elasticsearch.DataNode.JavaOpts != "" {
		javaOpts = vmi.Spec.Elasticsearch.DataNode.JavaOpts
	}

	initialMasterNodes := make([]string, 0)
	masterReplicas := resources.NewVal(vmi.Spec.Elasticsearch.MasterNode.Replicas)
	var i int32
	for i = 0; i < *masterReplicas; i++ {
		initialMasterNodes = append(initialMasterNodes, resources.GetMetaName(vmi.Name, config.ElasticsearchMaster.Name)+"-"+fmt.Sprintf("%d", i))
	}
	var deployList []*appsv1.Deployment
	for i := 0; i < int(vmi.Spec.Elasticsearch.DataNode.Replicas); i++ {
		elasticsearchDataDeployment := es.createElasticsearchCommonDeployment(vmi, &vmi.Spec.Elasticsearch.Storage, &vmi.Spec.Elasticsearch.DataNode.Resources, config.ElasticsearchData, i)

		elasticsearchDataDeployment.Spec.Replicas = resources.NewVal(1)
		availabilityDomain := getAvailabilityDomainForPvcIndex(&vmi.Spec.Elasticsearch.Storage, pvcToAdMap, i)
		if availabilityDomain == "" {
			// With shard allocation awareness, we must provide something for the AD, even in the case of the simple
			// VMI with no persistence volumes
			availabilityDomain = "None"
		}

		// Anti-affinity on other data pod *nodes* (try out best to spread across many nodes)
		elasticsearchDataDeployment.Spec.Template.Spec.Affinity = &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: resources.GetSpecId(vmi.Name, config.ElasticsearchData.Name),
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			},
		}
		// When the deployment does not have a pod security context with an FSGroup attribute, any mounted volumes are
		// initially owned by root/root.  Previous versions of the ES image were run as "root", and chown'd the mounted
		// directory to "elasticsearch", but we don't want to run as "root".  The current ES image creates a group
		// "elasticsearch" (GID 1000), and a user "elasticsearch" (UID 1000) in that group.  When we provide FSGroup =
		// 1000 below, the volume is owned by root/elasticsearch, with permissions "rwxrwsr-x".  This allows the ES
		// image to run as UID 1000, and have sufficient permissions to write to the mounted volume.
		elasticsearchGid := int64(1000)
		elasticsearchDataDeployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			FSGroup: &elasticsearchGid,
		}

		elasticsearchDataDeployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
		elasticsearchDataDeployment.Spec.Strategy.RollingUpdate = nil
		elasticsearchDataDeployment.Spec.Template.Spec.Containers[0].Env = append(elasticsearchDataDeployment.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{Name: "discovery.seed_hosts", Value: resources.GetMetaName(vmi.Name, config.ElasticsearchMaster.Name)},
			corev1.EnvVar{Name: "cluster.initial_master_nodes", Value: strings.Join(initialMasterNodes, ",")},
			corev1.EnvVar{Name: "node.attr.availability_domain", Value: availabilityDomain},
			corev1.EnvVar{Name: "node.master", Value: "false"},
			corev1.EnvVar{Name: "node.ingest", Value: "false"},
			corev1.EnvVar{Name: "node.data", Value: "true"},
			corev1.EnvVar{Name: "ES_JAVA_OPTS", Value: javaOpts},
		)

		// Add a node exporter container
		elasticsearchDataDeployment.Spec.Template.Spec.Containers = append(elasticsearchDataDeployment.Spec.Template.Spec.Containers, corev1.Container{
			Name:            config.NodeExporter.Name,
			Image:           config.NodeExporter.Image,
			ImagePullPolicy: constants.DefaultImagePullPolicy,
		})
		volumeMounts := []corev1.VolumeMount{
			{
				Name:      constants.StorageVolumeName,
				MountPath: constants.ElasticSearchNodeExporterPath,
			},
		}
		if vmi.Spec.Elasticsearch.Storage.Size != "" {
			elasticsearchDataDeployment.Spec.Template.Spec.Containers[1].VolumeMounts = volumeMounts
		}

		deployList = append(deployList, elasticsearchDataDeployment)
	}
	return deployList
}

// Creates *all* Elasticsearch deployment elements
func (es ElasticsearchBasic) createElasticsearchDeploymentElements(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment {
	var deployList []*appsv1.Deployment
	deployList = append(deployList, es.createElasticsearchIngestDeploymentElements(vmi)...)
	deployList = append(deployList, es.createElasticsearchDataDeploymentElements(vmi, pvcToAdMap)...)
	return deployList
}
