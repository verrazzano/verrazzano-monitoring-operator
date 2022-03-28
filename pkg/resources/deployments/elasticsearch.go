// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"fmt"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/memory"
	"go.uber.org/zap"
	"strings"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ElasticsearchBasic function type
type ElasticsearchBasic struct {
}

// Returns a common base deployment structure for all Elasticsearch components
func (es ElasticsearchBasic) createElasticsearchCommonDeployment(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, vmoStorage *vmcontrollerv1.Storage,
	vmoResources *vmcontrollerv1.Resources, componentDetails config.ComponentDetails, index int) *appsv1.Deployment {

	deploymentElement := createDeploymentElementByPvcIndex(vmo, vmoStorage, vmoResources, componentDetails, index)

	esContainer := &deploymentElement.Spec.Template.Spec.Containers[0]
	esContainer.Env = append(esContainer.Env,
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
		corev1.EnvVar{Name: "cluster.name", Value: vmo.Name},
		corev1.EnvVar{Name: "logger.org.opensearch", Value: "info"},
	)

	esContainer.Ports = []corev1.ContainerPort{
		{Name: "http", ContainerPort: int32(constants.ESHttpPort)},
		{Name: "transport", ContainerPort: int32(constants.ESTransportPort)},
	}

	// Common Elasticsearch readiness and liveness settings
	if esContainer.LivenessProbe != nil {
		esContainer.LivenessProbe.InitialDelaySeconds = 60
		esContainer.LivenessProbe.TimeoutSeconds = 3
		esContainer.LivenessProbe.PeriodSeconds = 20
		esContainer.LivenessProbe.FailureThreshold = 5
	}
	if esContainer.ReadinessProbe != nil {
		esContainer.ReadinessProbe.InitialDelaySeconds = 60
		esContainer.ReadinessProbe.TimeoutSeconds = 3
		esContainer.ReadinessProbe.PeriodSeconds = 10
		esContainer.ReadinessProbe.FailureThreshold = 10
	}

	// Add init containers
	deploymentElement.Spec.Template.Spec.InitContainers = append(deploymentElement.Spec.Template.Spec.InitContainers, *resources.GetElasticsearchInitContainer())

	// Istio does not work with ElasticSearch.  Uncomment the following line when istio is present
	// deploymentElement.Spec.Template.Annotations = map[string]string{"sidecar.istio.io/inject": "false"}

	var elasticsearchUID int64 = 1000
	esContainer.SecurityContext.RunAsUser = &elasticsearchUID
	return deploymentElement
}

// Creates all Elasticsearch Client deployment elements
func (es ElasticsearchBasic) createElasticsearchIngestDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []*appsv1.Deployment {
	// Default JVM heap settings if none provided
	javaOpts, err := memory.PodMemToJvmHeapArgs(vmo.Spec.Elasticsearch.IngestNode.Resources.RequestMemory)
	if err != nil {
		javaOpts = constants.DefaultESIngestMemArgs
		zap.S().Errorf("Failed to derive heap sizes from Ingest pod, using default %s: %v", javaOpts, err)
	}
	if vmo.Spec.Elasticsearch.IngestNode.JavaOpts != "" {
		javaOpts = vmo.Spec.Elasticsearch.IngestNode.JavaOpts
	}

	elasticsearchIngestDeployment := es.createElasticsearchCommonDeployment(vmo, nil, &vmo.Spec.Elasticsearch.IngestNode.Resources, config.ElasticsearchIngest, -1)

	elasticsearchIngestDeployment.Spec.Replicas = resources.NewVal(vmo.Spec.Elasticsearch.IngestNode.Replicas)

	// Anti-affinity on other client zones
	elasticsearchIngestDeployment.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.ElasticsearchIngest.Name)

	initialMasterNodes := make([]string, 0)
	masterReplicas := resources.NewVal(vmo.Spec.Elasticsearch.MasterNode.Replicas)
	var i int32
	for i = 0; i < *masterReplicas; i++ {
		initialMasterNodes = append(initialMasterNodes, resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)+"-"+fmt.Sprintf("%d", i))
	}
	elasticsearchIngestDeployment.Spec.Template.Spec.Containers[0].Env = append(elasticsearchIngestDeployment.Spec.Template.Spec.Containers[0].Env,
		corev1.EnvVar{Name: "discovery.seed_hosts", Value: resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)},
		corev1.EnvVar{Name: "cluster.initial_master_nodes", Value: strings.Join(initialMasterNodes, ",")},
		corev1.EnvVar{Name: "node.master", Value: "false"},
		corev1.EnvVar{Name: "NETWORK_HOST", Value: "0.0.0.0"},
		corev1.EnvVar{Name: "node.ingest", Value: "true"},
		corev1.EnvVar{Name: "node.data", Value: "false"},
		corev1.EnvVar{Name: "ES_JAVA_OPTS", Value: javaOpts},
	)
	// add the required istio annotations to allow inter-es component communication
	if elasticsearchIngestDeployment.Spec.Template.Annotations == nil {
		elasticsearchIngestDeployment.Spec.Template.Annotations = make(map[string]string)
	}
	elasticsearchIngestDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeInboundPorts"] = fmt.Sprintf("%d", constants.ESTransportPort)
	elasticsearchIngestDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeOutboundPorts"] = fmt.Sprintf("%d", constants.ESTransportPort)

	return []*appsv1.Deployment{elasticsearchIngestDeployment}
}

// Creates all Elasticsearch Data deployment elements
func (es ElasticsearchBasic) createElasticsearchDataDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment {
	// Default JVM heap settings if none provided
	javaOpts, err := memory.PodMemToJvmHeapArgs(vmo.Spec.Elasticsearch.DataNode.Resources.RequestMemory)
	if err != nil {
		javaOpts = constants.DefaultESDataMemArgs
		zap.S().Errorf("Failed to derive heap sizes from Data pod, using default %s: %v", javaOpts, err)
	}
	if vmo.Spec.Elasticsearch.DataNode.JavaOpts != "" {
		javaOpts = vmo.Spec.Elasticsearch.DataNode.JavaOpts
	}

	initialMasterNodes := make([]string, 0)
	masterReplicas := resources.NewVal(vmo.Spec.Elasticsearch.MasterNode.Replicas)
	var i int32
	for i = 0; i < *masterReplicas; i++ {
		initialMasterNodes = append(initialMasterNodes, resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)+"-"+fmt.Sprintf("%d", i))
	}
	var deployList []*appsv1.Deployment
	for i := 0; i < int(vmo.Spec.Elasticsearch.DataNode.Replicas); i++ {
		elasticsearchDataDeployment := es.createElasticsearchCommonDeployment(vmo, vmo.Spec.Elasticsearch.DataNode.Storage, &vmo.Spec.Elasticsearch.DataNode.Resources, config.ElasticsearchData, i)

		elasticsearchDataDeployment.Spec.Replicas = resources.NewVal(1)
		availabilityDomain := getAvailabilityDomainForPvcIndex(vmo.Spec.Elasticsearch.DataNode.Storage, pvcToAdMap, i)
		if availabilityDomain == "" {
			// With shard allocation awareness, we must provide something for the AD, even in the case of the simple
			// VMO with no persistence volumes
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
								MatchLabels: resources.GetSpecID(vmo.Name, config.ElasticsearchData.Name),
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
			corev1.EnvVar{Name: "discovery.seed_hosts", Value: resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)},
			corev1.EnvVar{Name: "cluster.initial_master_nodes", Value: strings.Join(initialMasterNodes, ",")},
			corev1.EnvVar{Name: "node.attr.availability_domain", Value: availabilityDomain},
			corev1.EnvVar{Name: "node.master", Value: "false"},
			corev1.EnvVar{Name: "node.ingest", Value: "false"},
			corev1.EnvVar{Name: "node.data", Value: "true"},
			corev1.EnvVar{Name: "ES_JAVA_OPTS", Value: javaOpts},
			corev1.EnvVar{Name: constants.ObjectStoreAccessKeyVarName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: constants.VerrazzanoBackupScrtName,
						},
						Key: constants.ObjectStoreAccessKey,
						Optional: func(opt bool) *bool {
							return &opt
						}(true),
					},
				},
			},
			corev1.EnvVar{Name: constants.ObjectStoreCustomerKeyVarName,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: constants.VerrazzanoBackupScrtName,
						},
						Key: constants.ObjectStoreCustomerKey,
						Optional: func(opt bool) *bool {
							return &opt
						}(true),
					},
				},
			},
		)

		// Adding command for add keystore values at pod bootup
		elasticsearchDataDeployment.Spec.Template.Spec.Containers[0].Command = []string{
			"sh",
			"-c",
			`#!/usr/bin/env bash -e

# Updating elastic search keystore with keys
# required for the repository-s3 plugin

if [ "${OBJECT_STORE_ACCESS_KEY_ID:-}" ]; then
    echo "Updating object store access key..."
	echo $OBJECT_STORE_ACCESS_KEY_ID | /usr/share/opensearch/bin/opensearch-keystore add --stdin --force s3.client.default.access_key;
fi
if [ "${OBJECT_STORE_SECRET_KEY_ID:-}" ]; then
    echo "Updating object store secret key..."
	echo $OBJECT_STORE_SECRET_KEY_ID | /usr/share/opensearch/bin/opensearch-keystore add --stdin --force s3.client.default.secret_key;
fi
/usr/local/bin/docker-entrypoint.sh`,
		}

		// add the required istio annotations to allow inter-es component communication
		if elasticsearchDataDeployment.Spec.Template.Annotations == nil {
			elasticsearchDataDeployment.Spec.Template.Annotations = make(map[string]string)
		}
		elasticsearchDataDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeInboundPorts"] = fmt.Sprintf("%d", constants.ESTransportPort)
		elasticsearchDataDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeOutboundPorts"] = fmt.Sprintf("%d", constants.ESTransportPort)

		deployList = append(deployList, elasticsearchDataDeployment)
	}
	return deployList
}

// Creates *all* Elasticsearch deployment elements
func (es ElasticsearchBasic) createElasticsearchDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment {
	var deployList []*appsv1.Deployment
	deployList = append(deployList, es.createElasticsearchIngestDeploymentElements(vmo)...)
	deployList = append(deployList, es.createElasticsearchDataDeploymentElements(vmo, pvcToAdMap)...)
	return deployList
}
