// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
			"--storage.tsdb.no-lockfile",
			"--log.level=debug"}

		// Not strictly necessary, but makes debugging easier to have a trace of the AD in the deployment itself
		env := prometheusDeployment.Spec.Template.Spec.Containers[0].Env
		env = append(env, corev1.EnvVar{Name: "AVAILABILITY_DOMAIN", Value: getAvailabilityDomainForPvcIndex(&vmo.Spec.Prometheus.Storage, pvcToAdMap, i)})
		prometheusDeployment.Spec.Template.Spec.Containers[0].Env = env

		setIstioAnnotations(prometheusDeployment)

		// Volumes for Prometheus config and alert rules.  The istio-certs-dir volume supports the output of the istio
		// certs for use by prometheus scrape configurations
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
			{
				Name: "istio-certs-dir",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{
						Medium: corev1.StorageMediumMemory,
					},
				},
			},
		}
		prometheusDeployment.Spec.Template.Spec.Volumes = append(prometheusDeployment.Spec.Template.Spec.Volumes, configVolumes...)
		configVolumeMounts := []corev1.VolumeMount{
			{
				Name:      "rules-volume",
				MountPath: constants.PrometheusRulesMountPath,
			},
			{
				Name:      "config-volume",
				MountPath: constants.PrometheusConfigMountPath,
			},
		}
		prometheusDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(prometheusDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, configVolumeMounts...)
		istioVolumeMount := corev1.VolumeMount{
			Name:      "istio-certs-dir",
			MountPath: constants.IstioCertsMountPath,
		}
		prometheusDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(prometheusDeployment.Spec.Template.Spec.Containers[0].VolumeMounts, istioVolumeMount)

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

		if config.Prometheus.OidcProxy != nil {
			oidcVolumes, oidcProxy := resources.CreateOidcProxy(vmo, &vmo.Spec.Prometheus.Resources, &config.Prometheus)
			prometheusDeployment.Spec.Template.Spec.Volumes = append(prometheusDeployment.Spec.Template.Spec.Volumes, oidcVolumes...)
			prometheusDeployment.Spec.Template.Spec.Containers = append(prometheusDeployment.Spec.Template.Spec.Containers, *oidcProxy)
		}

		prometheusNodeDeployments = append(prometheusNodeDeployments, prometheusDeployment)
	}
	return prometheusNodeDeployments
}

// setIstioAnnotations applies the annotations to ensure that:
// 1. Istio outputs its certs to the designated volume mount
// 2. The istio proxy only intercepts traffic bound for auth/keycloak and ignores scrape targets (only traffic for port
//    8443 is intercepted
func setIstioAnnotations(prometheusDeployment *appsv1.Deployment) {
	if prometheusDeployment.Spec.Template.Annotations == nil {
		prometheusDeployment.Spec.Template.Annotations = make(map[string]string)
	}
	// these annotation are required uniquely for prometheus to support both the request routing to keycloak via the envoy and the writing
	// of the istio certs to a volume that can be accessed for scraping
	prometheusDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/includeOutboundPorts"] = "8443"
	prometheusDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeOutboundPorts"] = "80,443,7001,8001,8080,9612,15090"
	prometheusDeployment.Spec.Template.Annotations["proxy.istio.io/config"] = `{"proxyMetadata":{ "OUTPUT_CERTS": "/etc/istio-output-certs"}}`
	prometheusDeployment.Spec.Template.Annotations["sidecar.istio.io/userVolumeMount"] = `[{"name": "istio-certs-dir", "mountPath": "/etc/istio-output-certs"}]`
	prometheusDeployment.Spec.Template.Annotations["sidecar.istio.io/logLevel"] = "debug"
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
