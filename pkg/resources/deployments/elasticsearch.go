// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/memory"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ElasticsearchBasic function type
type ElasticsearchBasic struct {
}

func IsOpenSearchDataDeployment(vmoName string, deployment *appsv1.Deployment) bool {
	return deployment.Spec.Template.Labels[constants.ServiceAppLabel] == vmoName+"-"+config.ElasticsearchData.Name
}

// Returns a common base deployment structure for all Elasticsearch components
func (es ElasticsearchBasic) createCommonDeployment(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, node vmcontrollerv1.ElasticsearchNode, componentDetails config.ComponentDetails, index int) *appsv1.Deployment {

	deploymentElement := createDeploymentElementByPvcIndex(vmo, node.Storage, &node.Resources, componentDetails, index, node.Name)
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
		{Name: "http", ContainerPort: int32(constants.OSHTTPPort)},
		{Name: "transport", ContainerPort: int32(constants.OSTransportPort)},
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

	// Add node labels
	deploymentElement.Spec.Selector.MatchLabels[constants.NodeGroupLabel] = node.Name
	deploymentElement.Spec.Template.Labels[constants.NodeGroupLabel] = node.Name
	nodes.SetNodeRoleLabels(&node, deploymentElement.Labels)
	nodes.SetNodeRoleLabels(&node, deploymentElement.Spec.Template.Labels)

	var elasticsearchUID int64 = 1000
	esContainer.SecurityContext.RunAsUser = &elasticsearchUID
	return deploymentElement
}

// Creates all Elasticsearch Client deployment elements
func (es ElasticsearchBasic) createElasticsearchIngestDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []*appsv1.Deployment {
	var deployments []*appsv1.Deployment
	nodeList := nodes.IngestNodes(vmo)
	for i, node := range nodeList {
		if node.Replicas < 1 {
			continue
		}
		// Default JVM heap settings if none provided
		javaOpts, err := memory.PodMemToJvmHeapArgs(node.Resources.RequestMemory, constants.DefaultESIngestMemArgs)
		if err != nil {
			javaOpts = constants.DefaultESIngestMemArgs
			zap.S().Errorf("Failed to derive heap sizes from IngestNodes pod, using default %s: %v", javaOpts, err)
		}
		if node.JavaOpts != "" {
			javaOpts = node.JavaOpts
		}

		ingestDeployment := es.createCommonDeployment(vmo, node, config.ElasticsearchIngest, -1)
		ingestDeployment.Spec.Replicas = resources.NewVal(node.Replicas)

		// Anti-affinity on other client zones
		ingestDeployment.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.ElasticsearchIngest.Name)
		ingestDeployment.Spec.Template.Spec.Containers[0].Env = append(ingestDeployment.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{Name: "discovery.seed_hosts", Value: resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)},
			corev1.EnvVar{Name: "NETWORK_HOST", Value: "0.0.0.0"},
			corev1.EnvVar{Name: "node.roles", Value: nodes.GetRolesString(&nodeList[i])},
			corev1.EnvVar{Name: "ES_JAVA_OPTS", Value: javaOpts},
		)
		// add the required istio annotations to allow inter-es component communication
		if ingestDeployment.Spec.Template.Annotations == nil {
			ingestDeployment.Spec.Template.Annotations = make(map[string]string)
		}
		ingestDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeInboundPorts"] = fmt.Sprintf("%d", constants.OSTransportPort)
		ingestDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeOutboundPorts"] = fmt.Sprintf("%d", constants.OSTransportPort)
		deployments = append(deployments, ingestDeployment)
	}
	return deployments
}

// Creates all Elasticsearch DataNodes deployment elements
func (es ElasticsearchBasic) createElasticsearchDataDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment {
	var deployments []*appsv1.Deployment
	nodeList := nodes.DataNodes(vmo)
	for idx, node := range nodeList {
		if node.Replicas < 1 {
			continue
		}
		// Default JVM heap settings if none provided
		javaOpts, err := memory.PodMemToJvmHeapArgs(node.Resources.RequestMemory, constants.DefaultESDataMemArgs)
		if err != nil {
			javaOpts = constants.DefaultESDataMemArgs
			zap.S().Errorf("Failed to derive heap sizes from DataNodes pod, using default %s: %v", javaOpts, err)
		}
		if node.JavaOpts != "" {
			javaOpts = node.JavaOpts
		}
		for i := 0; i < int(node.Replicas); i++ {
			dataDeployment := es.createCommonDeployment(vmo, node, config.ElasticsearchData, i)

			dataDeployment.Spec.Replicas = resources.NewVal(1)
			availabilityDomain := getAvailabilityDomainForPvcIndex(node.Storage, pvcToAdMap, i)
			if availabilityDomain == "" {
				// With shard allocation awareness, we must provide something for the AD, even in the case of the simple
				// VMO with no persistence volumes
				availabilityDomain = "None"
			}

			// Anti-affinity on other data pod *nodes* (try out best to spread across many nodes)
			dataDeployment.Spec.Template.Spec.Affinity = &corev1.Affinity{
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
			dataDeployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
				FSGroup: &elasticsearchGid,
			}

			dataDeployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
			dataDeployment.Spec.Strategy.RollingUpdate = nil
			dataDeployment.Spec.Template.Spec.Containers[0].Env = append(dataDeployment.Spec.Template.Spec.Containers[0].Env,
				corev1.EnvVar{Name: "discovery.seed_hosts", Value: resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name)},
				corev1.EnvVar{Name: "node.attr.availability_domain", Value: availabilityDomain},
				corev1.EnvVar{Name: "node.roles", Value: nodes.GetRolesString(&nodeList[idx])},
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
			dataDeployment.Spec.Template.Spec.Containers[0].Command = []string{
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
			if dataDeployment.Spec.Template.Annotations == nil {
				dataDeployment.Spec.Template.Annotations = make(map[string]string)
			}
			dataDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeInboundPorts"] = fmt.Sprintf("%d", constants.OSTransportPort)
			dataDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeOutboundPorts"] = fmt.Sprintf("%d", constants.OSTransportPort)
			deployments = append(deployments, dataDeployment)
		}
	}
	return deployments
}

// Creates *all* Elasticsearch deployment elements
func (es ElasticsearchBasic) createElasticsearchDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment {
	var deployList []*appsv1.Deployment
	deployList = append(deployList, es.createElasticsearchIngestDeploymentElements(vmo)...)
	deployList = append(deployList, es.createElasticsearchDataDeploymentElements(vmo, pvcToAdMap)...)
	return deployList
}
