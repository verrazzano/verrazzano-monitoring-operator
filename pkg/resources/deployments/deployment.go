// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Elasticsearch interface
type Elasticsearch interface {
	createElasticsearchDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment
}

// New function creates deployment objects for a VMO resource.  It also sets the appropriate OwnerReferences on
// the resource so handleObject can discover the VMO resource that 'owns' it.
func New(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, kubeclientset kubernetes.Interface, operatorConfig *config.OperatorConfig, pvcToAdMap map[string]string, username string, password string) ([]*appsv1.Deployment, error) {
	var deployments []*appsv1.Deployment
	var err error

	// Elasticsearch
	// - V8O supports essentially 2 "known" configurations, a "prod" and a "dev" configuration for ES; while we want
	//   to allow customizing topologies, we need to enforce certain constraints for now.
	// - We are arbitrarily choosing to enforce that a "valid" multi-node cluster includes at least one separate
	//   data node and one separate ingest node
	// - This will weed out creating separate pods for data/ingest in the single-node cluster configuration as well
	if vmo.Spec.Elasticsearch.Enabled {
		if resources.IsValidMultiNodeESCluster(vmo) {
			var es Elasticsearch = ElasticsearchBasic{}
			deployments = append(deployments, es.createElasticsearchDeploymentElements(vmo, pvcToAdMap)...)
		} else if !resources.IsSingleNodeESCluster(vmo) {
			err = errors.New("Invalid Elasticsearch cluster configuration, must be a valid single or multi-node cluster configuration")
		}
	}

	// Kibana
	if vmo.Spec.Kibana.Enabled {
		elasticsearchURL := fmt.Sprintf("http://%s%s-%s:%d/", constants.VMOServiceNamePrefix, vmo.Name, config.ElasticsearchIngest.Name, config.ElasticsearchIngest.Port)
		deployment := createDeploymentElement(vmo, nil, &vmo.Spec.Kibana.Resources, config.Kibana)

		deployment.Spec.Replicas = resources.NewVal(vmo.Spec.Kibana.Replicas)
		deployment.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.Kibana.Name)
		deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
			{Name: "ELASTICSEARCH_HOSTS", Value: elasticsearchURL},
		}

		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = 120
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds = 3
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds = 20
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold = 10

		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds = 15
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds = 3
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds = 20
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold = 5

		// add the required istio annotations to allow inter-es component communication
		if deployment.Spec.Template.Annotations == nil {
			deployment.Spec.Template.Annotations = make(map[string]string)
		}
		deployment.Spec.Template.Annotations["traffic.sidecar.istio.io/includeOutboundPorts"] = fmt.Sprintf("%d", constants.ESHttpPort)

		deployments = append(deployments, deployment)
	}

	// API
	if !config.API.Disabled {
		deployment := createDeploymentElement(vmo, nil, nil, config.API)
		deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.API.ImagePullPolicy
		deployment.Spec.Replicas = resources.NewVal(vmo.Spec.API.Replicas)
		deployment.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.API.Name)
		deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
			{Name: "VMI_NAME", Value: vmo.Name},
			{Name: "NAMESPACE", Value: vmo.Namespace},
			{Name: "ENV_NAME", Value: operatorConfig.EnvName},
		}
		if len(vmo.Spec.NatGatewayIPs) > 0 {
			deployment.Spec.Template.Spec.Containers[0].Args = []string{fmt.Sprintf("--natGatewayIPs=%s", strings.Join(vmo.Spec.NatGatewayIPs, ","))}
		}

		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = 15
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds = 3
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds = 5
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds = 3

		deployments = append(deployments, deployment)
	}

	return deployments, err
}

func createVolumeElement(pvcName string) corev1.Volume {
	return corev1.Volume{
		Name: constants.StorageVolumeName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
				ReadOnly:  false,
			},
		},
	}
}

// Creates a deployment element for the given VMO and component.
func createDeploymentElement(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, vmoStorage *vmcontrollerv1.Storage,
	vmoResources *vmcontrollerv1.Resources, componentDetails config.ComponentDetails) *appsv1.Deployment {
	return createDeploymentElementByPvcIndex(vmo, vmoStorage, vmoResources, componentDetails, -1)
}

// Creates a deployment element for the given VMO and component.  A non-negative pvcIndex is used to indicate which
// PVC in the list of PVCs should be used for this particular deployment.
func createDeploymentElementByPvcIndex(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, vmoStorage *vmcontrollerv1.Storage,
	vmoResources *vmcontrollerv1.Resources, componentDetails config.ComponentDetails, pvcIndex int) *appsv1.Deployment {

	labels := resources.GetSpecID(vmo.Name, componentDetails.Name)
	var deploymentName string
	if pvcIndex < 0 {
		deploymentName = resources.GetMetaName(vmo.Name, componentDetails.Name)
		pvcIndex = 0
	} else {
		deploymentName = resources.GetMetaName(vmo.Name, fmt.Sprintf("%s-%d", componentDetails.Name, pvcIndex))
	}

	var volumes []corev1.Volume
	if vmoStorage != nil && vmoStorage.Size != "" {
		// Create volume element for this component, attaching to that component's current known PVC (if set)
		volumes = append(volumes, createVolumeElement(vmoStorage.PvcNames[pvcIndex]))
		labels["index"] = strconv.Itoa(pvcIndex)
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resources.GetMetaLabels(vmo),
			Name:            deploymentName,
			Namespace:       vmo.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmo),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: resources.NewVal(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Volumes: volumes,
					Containers: []corev1.Container{
						resources.CreateContainerElement(vmoStorage, vmoResources, componentDetails),
					},
					ServiceAccountName:            constants.ServiceAccountName,
					TerminationGracePeriodSeconds: resources.New64Val(1),
				},
			},
		},
	}
}

// Helper function that returns the AD name for the PVC at the given index in the given Storage element.  Under any
// error condition, an empty string is returned.
func getAvailabilityDomainForPvcIndex(vmoStorage *vmcontrollerv1.Storage, pvcToAdMap map[string]string, pvcIndex int) string {
	if vmoStorage == nil || pvcIndex > len(vmoStorage.PvcNames)-1 || pvcIndex < 0 {
		return ""
	}
	if ad, ok := pvcToAdMap[vmoStorage.PvcNames[pvcIndex]]; ok {
		return ad
	}
	return ""
}
