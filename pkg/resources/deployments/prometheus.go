// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"fmt"
	"os"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Creates Prometheus node deployment elements
func createPrometheusNodeDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment {
	var prometheusNodeDeployments []*appsv1.Deployment
	for i := 0; i < int(vmo.Spec.Prometheus.Replicas); i++ {
		prometheusDeployment := createDeploymentElementByPvcIndex(vmo, &vmo.Spec.Prometheus.Storage, &vmo.Spec.Prometheus.Resources, config.Prometheus, i)

		prometheusDeployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType

		// Main Prometheus parameters
		prometheusDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.Prometheus.ImagePullPolicy
		prometheusDeployment.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = &config.Prometheus.RunAsUser

		prometheusDeployment.Spec.Template.Spec.Containers[0].Command = []string{"/bin/prometheus"}
		prometheusDeployment.Spec.Template.Spec.Containers[0].Args = []string{
			"--config.file=" + constants.PrometheusConfigContainerLocation,
			"--storage.tsdb.path=" + config.Prometheus.DataDir,
			fmt.Sprintf("--storage.tsdb.retention.time=%dd", vmo.Spec.Prometheus.RetentionPeriod),
			"--web.enable-lifecycle",
			"--web.enable-admin-api",
			"--storage.tsdb.no-lockfile"}

		// Not strictly necessary, but makes debugging easier to have a trace of the AD in the deployment itself
		env := prometheusDeployment.Spec.Template.Spec.Containers[0].Env
		env = append(env, corev1.EnvVar{Name: "AVAILABILITY_DOMAIN", Value: getAvailabilityDomainForPvcIndex(&vmo.Spec.Prometheus.Storage, pvcToAdMap, i)})
		prometheusDeployment.Spec.Template.Spec.Containers[0].Env = env

		// Volumes for Prometheus config and alert rules
		configVolumes := []corev1.Volume{
			{
				Name: "rules-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.Prometheus.RulesConfigMap},
					},
				},
			},
			{
				Name: "config-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: vmo.Spec.Prometheus.ConfigMap},
					},
				},
			},
		}
		configVolumeMounts := []corev1.VolumeMount{
			{
				Name:      "rules-volume",
				MountPath: constants.PrometheusRulesMountPath,
			},
			{
				Name:      "config-volume",
				MountPath: constants.PrometheusConfigMountPath,
			},
			{
				Name:      "istio-certs",
				MountPath: constants.PrometheusIstioCertsMountPath,
			},
		}
		prometheusDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(prometheusDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, configVolumeMounts...)
		prometheusDeployment.Spec.Template.Spec.Volumes = append(prometheusDeployment.Spec.Template.Spec.Volumes, configVolumes...)

		// Readiness/liveness settings
		prometheusDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = 30
		prometheusDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds = 3
		prometheusDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds = 10
		prometheusDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold = 10
		prometheusDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds = 5
		prometheusDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds = 3
		prometheusDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds = 10
		prometheusDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold = 5

		// Config-reloader container
		prometheusDeployment.Spec.Template.Spec.Containers = append(prometheusDeployment.Spec.Template.Spec.Containers, corev1.Container{
			Name:            config.ConfigReloader.Name,
			Image:           config.ConfigReloader.Image,
			ImagePullPolicy: config.ConfigReloader.ImagePullPolicy,
		})
		prometheusDeployment.Spec.Template.Spec.Containers[1].Args = []string{"-volume-dir=" + constants.PrometheusConfigMountPath, "-volume-dir=" + constants.PrometheusRulesMountPath, "-webhook-url=http://localhost:9090/-/reload"}
		prometheusDeployment.Spec.Template.Spec.Containers[1].VolumeMounts = configVolumeMounts

		// istio-proxy container needed to scrape metrics with mutual TLS enabled
		prometheusDeployment.Spec.Template.Spec.Containers = append(prometheusDeployment.Spec.Template.Spec.Containers, *createPrometheusIstioProxyContainer())

		// Prometheus init container
		prometheusDeployment.Spec.Template.Spec.InitContainers = []corev1.Container{
			{
				Name:            config.PrometheusInit.Name,
				Image:           config.PrometheusInit.Image,
				ImagePullPolicy: config.PrometheusInit.ImagePullPolicy,
				Command:         []string{"sh", "-c", fmt.Sprintf("chown -R %d:%d /prometheus", constants.NobodyUID, constants.NobodyUID)},
				VolumeMounts:    []corev1.VolumeMount{{Name: constants.StorageVolumeName, MountPath: config.PrometheusInit.DataDir}},
			},
		}
		if vmo.Spec.Prometheus.Storage.Size == "" {
			prometheusDeployment.Spec.Template.Spec.Volumes = append(
				prometheusDeployment.Spec.Template.Spec.Volumes,
				corev1.Volume{Name: constants.StorageVolumeName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
			prometheusDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(
				prometheusDeployment.Spec.Template.Spec.Containers[0].VolumeMounts,
				corev1.VolumeMount{Name: constants.StorageVolumeName, MountPath: config.Prometheus.DataDir})
		}

		// Add the Istio volumes
		volume := corev1.Volume{
			Name: "istio-certs",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		}
		prometheusDeployment.Spec.Template.Spec.Volumes = append(prometheusDeployment.Spec.Template.Spec.Volumes, volume)

		volume = corev1.Volume{
			Name: "istio-envoy",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: corev1.StorageMediumMemory,
				},
			},
		}
		prometheusDeployment.Spec.Template.Spec.Volumes = append(prometheusDeployment.Spec.Template.Spec.Volumes, volume)

		var defaultMode int32 = 420
		var expirationSeconds int64 = 43200
		volume = corev1.Volume{
			Name: "istio-token",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: &defaultMode,
					Sources: []corev1.VolumeProjection{
						{ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Audience:          "istio-ca",
							ExpirationSeconds: &expirationSeconds,
							Path:              "istio-token",
						}},
					},
				},
			},
		}
		prometheusDeployment.Spec.Template.Spec.Volumes = append(prometheusDeployment.Spec.Template.Spec.Volumes, volume)

		volume = corev1.Volume{
			Name: "istiod-ca-cert",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &defaultMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "istio-ca-root-cert",
					},
				},
			},
		}
		prometheusDeployment.Spec.Template.Spec.Volumes = append(prometheusDeployment.Spec.Template.Spec.Volumes, volume)

		if config.Prometheus.OidcProxy != nil {
			oidcVolumes, oidcProxy := resources.CreateOidcProxy(vmo, &vmo.Spec.Prometheus.Resources, &config.Prometheus)
			prometheusDeployment.Spec.Template.Spec.Volumes = append(prometheusDeployment.Spec.Template.Spec.Volumes, oidcVolumes...)
			prometheusDeployment.Spec.Template.Spec.Containers = append(prometheusDeployment.Spec.Template.Spec.Containers, *oidcProxy)
		}

		prometheusNodeDeployments = append(prometheusNodeDeployments, prometheusDeployment)
	}
	return prometheusNodeDeployments
}

// Creates Prometheus istio-proxy container elements
func createPrometheusIstioProxyContainer() *corev1.Container {
	container := &corev1.Container{
		Name:            "istio-proxy",
		Image:           os.Getenv("ISTIO_PROXY_IMAGE"),
		ImagePullPolicy: constants.DefaultImagePullPolicy,
		Args: []string{
			"proxy",
			"sidecar",
			"--domain",
			"$(POD_NAMESPACE).svc.cluster.local",
			"--serviceCluster",
			"istio-proxy-prometheus",
			"--proxyLogLevel=warning",
			"--proxyComponentLogLevel=misc:error",
			"--trust-domain=cluster.local",
			"--concurrency",
			"2",
		},
		Env: []corev1.EnvVar{
			{
				Name:  "OUTPUT_CERTS",
				Value: "/etc/istio-certs",
			},
			{
				Name:  "JWT_POLICY",
				Value: "third-party-jwt",
			},
			{
				Name:  "CA_ADDR",
				Value: "istiod.istio-system.svc:15012",
			},
			{
				Name:  "PILOT_CERT_PROVIDER",
				Value: "istiod",
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "INSTANCE_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.podIP",
					},
				},
			},
			{
				Name: "SERVICE_ACCOUNT",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "spec.serviceAccountName",
					},
				},
			},
			{
				Name: "HOST_IP",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "status.hostIP",
					},
				},
			},
			{
				Name: "ISTIO_META_POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			{
				Name: "ISTIO_META_CONFIG_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name:  "ISTIO_META_MESH_ID",
				Value: "cluster.local",
			},
			{
				Name:  "ISTIO_META_CLUSTER_ID",
				Value: "Kubernetes",
			},
		},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 15090,
				Name:          "http-envoy-prom",
				Protocol:      corev1.ProtocolTCP,
			},
		},
		ReadinessProbe: &corev1.Probe{
			FailureThreshold: 30,
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz/ready",
					Port: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 15020,
					},
					Scheme: corev1.URISchemeHTTP,
				},
			},
			InitialDelaySeconds: 1,
			PeriodSeconds:       2,
			SuccessThreshold:    1,
			TimeoutSeconds:      1,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				MountPath: "/var/run/secrets/istio",
				Name:      "istiod-ca-cert",
			},
			{
				MountPath: "/etc/istio/proxy",
				Name:      "istio-envoy",
			},
			{
				MountPath: "/var/run/secrets/tokens",
				Name:      "istio-token",
			},
			{
				MountPath: constants.PrometheusIstioCertsMountPath,
				Name:      "istio-certs",
			},
		},
	}

	return container
}

// Creates Prometheus Push Gateway deployment element
func createPrometheusPushGatewayDeploymentElement(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *appsv1.Deployment {
	pushGatewayDeployment := createDeploymentElement(vmo, nil, &vmo.Spec.PrometheusGW.Resources, config.PrometheusGW)
	pushGatewayDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.PrometheusGW.ImagePullPolicy
	pushGatewayDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.InitialDelaySeconds = 5
	pushGatewayDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.TimeoutSeconds = 3
	pushGatewayDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.PeriodSeconds = 10
	pushGatewayDeployment.Spec.Template.Spec.Containers[0].LivenessProbe.FailureThreshold = 10

	pushGatewayDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.InitialDelaySeconds = 5
	pushGatewayDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.TimeoutSeconds = 3
	pushGatewayDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.PeriodSeconds = 10
	pushGatewayDeployment.Spec.Template.Spec.Containers[0].ReadinessProbe.FailureThreshold = 5

	return pushGatewayDeployment
}

// Creates *all* Prometheus-related deployment elements
func createPrometheusDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment {
	var deployList []*appsv1.Deployment
	deployList = append(deployList, createPrometheusNodeDeploymentElements(vmo, pvcToAdMap)...)
	deployList = append(deployList, createPrometheusPushGatewayDeploymentElement(vmo))
	return deployList
}
