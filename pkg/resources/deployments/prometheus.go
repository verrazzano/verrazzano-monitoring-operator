// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package deployments

import (
	"fmt"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// Creates Prometheus node deployment elements
func createPrometheusNodeDeploymentElements(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment {
	var prometheusNodeDeployments []*appsv1.Deployment
	for i := 0; i < int(vmi.Spec.Prometheus.Replicas); i++ {
		prometheusDeployment := createDeploymentElementByPvcIndex(vmi, &vmi.Spec.Prometheus.Storage, &vmi.Spec.Prometheus.Resources, config.Prometheus, i)
		prometheusDeployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType

		// Main Prometheus parameters
		prometheusDeployment.Spec.Template.Spec.Containers[0].ImagePullPolicy = config.Prometheus.ImagePullPolicy
		prometheusDeployment.Spec.Template.Spec.Containers[0].SecurityContext.RunAsUser = &config.Prometheus.RunAsUser

		prometheusDeployment.Spec.Template.Spec.Containers[0].Command = []string{"/bin/prometheus"}
		prometheusDeployment.Spec.Template.Spec.Containers[0].Args = []string{
			"--config.file=" + constants.PrometheusConfigContainerLocation,
			"--storage.tsdb.path=" + config.Prometheus.DataDir,
			fmt.Sprintf("--storage.tsdb.retention.time=%dd", vmi.Spec.Prometheus.RetentionPeriod),
			"--web.enable-lifecycle",
			"--web.enable-admin-api",
			"--storage.tsdb.no-lockfile"}

		// Not strictly necessary, but makes debugging easier to have a trace of the AD in the deployment itself
		env := prometheusDeployment.Spec.Template.Spec.Containers[0].Env
		env = append(env, corev1.EnvVar{Name: "AVAILABILITY_DOMAIN", Value: getAvailabilityDomainForPvcIndex(&vmi.Spec.Prometheus.Storage, pvcToAdMap, i)})
		prometheusDeployment.Spec.Template.Spec.Containers[0].Env = env

		// Volumes for Prometheus config and alert rules
		configVolumes := []corev1.Volume{
			{
				Name: "rules-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: vmi.Spec.Prometheus.RulesConfigMap},
					},
				},
			},
			{
				Name: "config-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: vmi.Spec.Prometheus.ConfigMap},
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

		// Node-exporter container
		prometheusDeployment.Spec.Template.Spec.Containers = append(prometheusDeployment.Spec.Template.Spec.Containers, corev1.Container{
			Name:            config.NodeExporter.Name,
			Image:           config.NodeExporter.Image,
			ImagePullPolicy: config.NodeExporter.ImagePullPolicy,
		})
		nodeExporterMount := []corev1.VolumeMount{
			{
				Name:      constants.StorageVolumeName,
				MountPath: constants.PrometheusNodeExporterPath,
			},
		}
		if vmi.Spec.Prometheus.Storage.Size != "" {
			prometheusDeployment.Spec.Template.Spec.Containers[2].VolumeMounts = nodeExporterMount
		}

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
		if vmi.Spec.Prometheus.Storage.Size == "" {
			prometheusDeployment.Spec.Template.Spec.Volumes = append(
				prometheusDeployment.Spec.Template.Spec.Volumes,
				corev1.Volume{Name: constants.StorageVolumeName, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
			prometheusDeployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(
				prometheusDeployment.Spec.Template.Spec.Containers[0].VolumeMounts,
				corev1.VolumeMount{Name: constants.StorageVolumeName, MountPath: config.Prometheus.DataDir})
		}

		prometheusNodeDeployments = append(prometheusNodeDeployments, prometheusDeployment)
	}
	return prometheusNodeDeployments
}

// Creates Prometheus Push Gateway deployment element
func createPrometheusPushGatewayDeploymentElement(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) *appsv1.Deployment {
	pushGatewayDeployment := createDeploymentElement(vmi, nil, &vmi.Spec.PrometheusGW.Resources, config.PrometheusGW)
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
func createPrometheusDeploymentElements(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, pvcToAdMap map[string]string) []*appsv1.Deployment {
	var deployList []*appsv1.Deployment
	deployList = append(deployList, createPrometheusNodeDeploymentElements(vmi, pvcToAdMap)...)
	deployList = append(deployList, createPrometheusPushGatewayDeploymentElement(vmi))
	return deployList
}
