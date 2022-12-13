// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
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
	createElasticsearchDataDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment
	createElasticsearchIngestDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []*appsv1.Deployment
}

type ExpectedDeployments struct {
	Deployments                 []*appsv1.Deployment
	GrafanaDeployments          int
	OpenSearchDataDeployments   int
	OpenSearchIngestDeployments int
}

// New function creates deployment objects for a VMO resource.  It also sets the appropriate OwnerReferences on
// the resource so handleObject can discover the VMO resource that 'owns' it.
func New(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, kubeclientset kubernetes.Interface, operatorConfig *config.OperatorConfig, pvcToAdMap map[string]string) (*ExpectedDeployments, error) {
	expected := &ExpectedDeployments{}
	var deployments []*appsv1.Deployment
	var err error

	if vmo.Spec.Elasticsearch.Enabled {
		basic := ElasticsearchBasic{}
		ingestDeployments := basic.createElasticsearchIngestDeploymentElements(vmo)
		dataDeployments := basic.createElasticsearchDataDeploymentElements(vmo, pvcToAdMap)
		deployments = append(deployments, ingestDeployments...)
		deployments = append(deployments, dataDeployments...)
		expected.OpenSearchIngestDeployments += len(ingestDeployments)
		expected.OpenSearchDataDeployments += len(dataDeployments)
	}

	// Grafana
	if vmo.Spec.Grafana.Enabled {
		expected.GrafanaDeployments++
		deployment := createDeploymentElement(vmo, &vmo.Spec.Grafana.Storage, &vmo.Spec.Grafana.Resources, config.Grafana, config.Grafana.Name)
		deployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.Grafana.ImagePullPolicy
		deployment.Spec.Replicas = resources.NewVal(vmo.Spec.Grafana.Replicas)
		deployment.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.Grafana.Name)

		deployment.Spec.Strategy.Type = "Recreate"
		deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
			{Name: "GF_PATHS_PROVISIONING", Value: "/etc/grafana/provisioning"},
			{Name: "GF_SERVER_ENABLE_GZIP", Value: "true"},
			{Name: "PROMETHEUS_TARGETS", Value: "http://" + constants.VMOServiceNamePrefix + vmo.Name + "-" + config.Prometheus.Name + ":" + strconv.Itoa(config.Prometheus.Port)},
		}
		if config.Grafana.OidcProxy == nil {
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, []corev1.EnvVar{
				{
					Name: "GF_SECURITY_ADMIN_USER",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: constants.GrafanaAdminSecret,
							},
							Key: constants.VMOSecretUsernameField,
						},
					},
				},
				{
					Name: "GF_SECURITY_ADMIN_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: constants.GrafanaAdminSecret,
							},
							Key: constants.VMOSecretPasswordField,
						},
					},
				},
				{Name: "GF_AUTH_ANONYMOUS_ENABLED", Value: "false"},
				{Name: "GF_AUTH_BASIC_ENABLED", Value: "true"},
				{Name: "GF_USERS_ALLOW_SIGN_UP", Value: "false"},
				{Name: "GF_USERS_AUTO_ASSIGN_ORG", Value: "true"},
				{Name: "GF_USERS_AUTO_ASSIGN_ORG_ROLE", Value: "Editor"},
				{Name: "GF_AUTH_DISABLE_LOGIN_FORM", Value: "false"},
				{Name: "GF_AUTH_DISABLE_SIGNOUT_MENU", Value: "false"},
			}...)
		} else {
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, []corev1.EnvVar{
				{
					Name: "GF_SECURITY_ADMIN_USER",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: constants.GrafanaAdminSecret,
							},
							Key: constants.VMOSecretUsernameField,
						},
					},
				},
				{
					Name: "GF_SECURITY_ADMIN_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: constants.GrafanaAdminSecret,
							},
							Key: constants.VMOSecretPasswordField,
						},
					},
				},
				{Name: "GF_AUTH_ANONYMOUS_ENABLED", Value: "false"},
				{Name: "GF_AUTH_BASIC_ENABLED", Value: "false"},
				{Name: "GF_USERS_ALLOW_SIGN_UP", Value: "false"},
				{Name: "GF_USERS_AUTO_ASSIGN_ORG", Value: "true"},
				{Name: "GF_USERS_AUTO_ASSIGN_ORG_ROLE", Value: "Editor"},
				{Name: "GF_AUTH_DISABLE_LOGIN_FORM", Value: "true"},
				{Name: "GF_AUTH_DISABLE_SIGNOUT_MENU", Value: "true"},
				{Name: "GF_AUTH_PROXY_ENABLED", Value: "true"},
				{Name: "GF_AUTH_PROXY_HEADER_NAME", Value: "X-WEBAUTH-USER"},
				{Name: "GF_AUTH_PROXY_HEADER_PROPERTY", Value: "username"},
				{Name: "GF_AUTH_PROXY_AUTO_SIGN_UP", Value: "true"},
			}...)
		}
		if vmo.Spec.Grafana.Database != nil {
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, []corev1.EnvVar{
				{
					Name: "GF_DATABASE_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: vmo.Spec.Grafana.Database.PasswordSecret,
							},
							Key: constants.VMOSecretPasswordField,
						},
					},
				},
				{
					Name: "GF_DATABASE_USER",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: vmo.Spec.Grafana.Database.PasswordSecret,
							},
							Key: constants.VMOSecretUsernameField,
						},
					},
				},
				{Name: "GF_DATABASE_HOST", Value: vmo.Spec.Grafana.Database.Host},
				{Name: "GF_DATABASE_TYPE", Value: "mysql"},
				{Name: "GF_DATABASE_NAME", Value: vmo.Spec.Grafana.Database.Name},
			}...)
		}
		if vmo.Spec.URI != "" {
			externalDomainName := config.Grafana.Name + "." + vmo.Spec.URI
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "GF_SERVER_DOMAIN", Value: externalDomainName})
			deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "GF_SERVER_ROOT_URL", Value: "https://" + externalDomainName})
		}
		// container will be restarted (per restart policy) if it fails the following liveness check:
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = 15
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds = 3
		deployment.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds = 20

		// container will be removed from services if fails the following readiness check.
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds = 5
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds = 3
		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds = 20

		deployment.Spec.Template.Spec.Containers[0].ReadinessProbe = deployment.Spec.Template.Spec.Containers[0].LivenessProbe

		// dashboard volume
		volumes := []corev1.Volume{
			{
				Name: "dashboards-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.Grafana.DashboardsConfigMap},
					},
				},
			},
			{
				Name: "datasources-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.Grafana.DatasourcesConfigMap},
					},
				},
			},
		}
		volumeMounts := []corev1.VolumeMount{
			{
				Name:      "dashboards-volume",
				MountPath: "/etc/grafana/provisioning/dashboards",
			},

			{
				Name:      "datasources-volume",
				MountPath: "/etc/grafana/provisioning/datasources",
			},
		}
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, volumeMounts...)
		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, volumes...)

		// Setup the sidecar for the dashboard creator
		for i, sidecar := range config.Grafana.Sidecars {
			if sidecar.Disabled {
				continue
			}
			deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, resources.CreateSidecarContainer(sidecar))
			deployment.Spec.Template.Spec.Containers[i+1].Env = append(deployment.Spec.Template.Spec.Containers[i+1].Env, []corev1.EnvVar{
				// These values are also used in the grafana-deployment helm chart in Verrazzano
				// This label allows us to select the correct dashboard ConfigMaps to be deployed in Grafana
				{Name: "LABEL", Value: "grafana_dashboard"},
				{Name: "LABEL_VALUE", Value: "\"1\""},
				{Name: "FOLDER", Value: "/etc/grafana/provisioning/dashboards"},
			}...)
			deployment.Spec.Template.Spec.Containers[i+1].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[i+1].VolumeMounts, corev1.VolumeMount{
				Name:      "dashboards-volume",
				MountPath: "/etc/grafana/provisioning/dashboards",
			})
		}

		// When the deployment does not have a pod security context with an FSGroup attribute, any mounted volumes are
		// initially owned by root/root.  Previous versions of the Grafana image were run as "root", and chown'd the mounted
		// directory to "grafana", but we don't want to run as "root".  The current Grafana image creates a group
		// "grafana" (GID 472), and a user "grafana" (UID 472) in that group.  When we provide FSGroup =
		// 472 below, the volume is owned by root/grafana, with permissions "rwxrwsr-x".  This allows the Grafana
		// image to run as UID 472, and have sufficient permissions to write to the mounted volume.
		grafanaGid := int64(472)
		deployment.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{
			FSGroup: &grafanaGid,
		}
		deployments = append(deployments, deployment)
	}

	// API
	if !config.API.Disabled {
		deployment := createDeploymentElement(vmo, nil, nil, config.API, config.API.Name)
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

	expected.Deployments = deployments
	return expected, err
}

func NewOpenSearchDashboardsDeployment(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *appsv1.Deployment {
	var deployment *appsv1.Deployment
	if vmo.Spec.Kibana.Enabled {
		opensearchURL := fmt.Sprintf("http://%s%s-%s:%d/", constants.VMOServiceNamePrefix, vmo.Name, config.OpensearchIngest.Name, config.OpensearchIngest.Port)

		deployment = createDeploymentElement(vmo, nil, &vmo.Spec.Kibana.Resources, config.OpenSearchDashboards, config.OpenSearchDashboards.Name)
		deployment.Spec.Strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		}
		deployment.Spec.Replicas = resources.NewVal(vmo.Spec.Kibana.Replicas)
		deployment.Spec.Template.Spec.Affinity = resources.CreateZoneAntiAffinityElement(vmo.Name, config.OpenSearchDashboards.Name)
		deployment.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
			{Name: "OPENSEARCH_HOSTS", Value: opensearchURL},
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
		deployment.Spec.Template.Annotations["traffic.sidecar.istio.io/includeOutboundPorts"] = fmt.Sprintf("%d", constants.OSHTTPPort)
		// Adding command to install OS plugins at pod bootup
		deployment.Spec.Template.Spec.Containers[0].Command = []string{
			"sh",
			"-c",
			fmt.Sprintf(resources.OpenSearchDashboardCmdTmpl, resources.GetOSPluginsInstallTmpl(resources.GetOSDashboardPluginList(vmo), resources.OSDashboardPluginsInstallCmd)),
		}
	}

	return deployment
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
	vmoResources *vmcontrollerv1.Resources, componentDetails config.ComponentDetails, name string) *appsv1.Deployment {
	return createDeploymentElementByPvcIndex(vmo, vmoStorage, vmoResources, componentDetails, -1, name)
}

// Creates a deployment element for the given VMO and component.  A non-negative pvcIndex is used to indicate which
// PVC in the list of PVCs should be used for this particular deployment.
func createDeploymentElementByPvcIndex(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, vmoStorage *vmcontrollerv1.Storage,
	vmoResources *vmcontrollerv1.Resources, componentDetails config.ComponentDetails, pvcIndex int, name string) *appsv1.Deployment {

	labels := resources.GetSpecID(vmo.Name, componentDetails.Name)
	var deploymentName string
	if pvcIndex < 0 {
		deploymentName = resources.GetMetaName(vmo.Name, name)
		pvcIndex = 0
	} else {
		deploymentName = resources.GetMetaName(vmo.Name, fmt.Sprintf("%s-%d", name, pvcIndex))
	}

	var volumes []corev1.Volume
	if vmoStorage != nil && vmoStorage.PvcNames != nil && vmoStorage.Size != "" {
		// Create volume element for this component, attaching to that component's current known PVC (if set)
		volumes = append(volumes, createVolumeElement(vmoStorage.PvcNames[pvcIndex]))
		labels["index"] = strconv.Itoa(pvcIndex)
	}

	resourceLabel := resources.GetMetaLabels(vmo)
	resourceLabel[constants.ComponentLabel] = resources.GetCompLabel(componentDetails.Name)
	podLabels := resources.DeepCopyMap(labels)
	podLabels[constants.ComponentLabel] = resources.GetCompLabel(componentDetails.Name)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resourceLabel,
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
					Labels: podLabels,
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
