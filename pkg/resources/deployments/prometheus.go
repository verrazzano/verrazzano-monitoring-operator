// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"context"
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Creates Prometheus node deployment elements
func createPrometheusNodeDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, dynamicclientset dynamic.Interface, pvcToAdMap map[string]string) ([]*appsv1.Deployment, error) {
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

		err := setIstioAnnotations(prometheusDeployment, dynamicclientset)
		if err != nil {
			return nil, err
		}

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
	return prometheusNodeDeployments, nil
}

// setIstioAnnotations applies the annotations to ensure that:
// 1. Istio outputs its certs to the designated volume mount
// 2. The Istio proxy only intercepts traffic bound for auth/keycloak and ignores scrape targets (only traffic for port
//    8443 is intercepted
// 3. The Istio includeOutboundIPRanges and excludeOutboundIPRanges are set based on whether keycloak is enabled or not
func setIstioAnnotations(prometheusDeployment *appsv1.Deployment, dynamicclientset dynamic.Interface) error {
	if prometheusDeployment.Spec.Template.Annotations == nil {
		prometheusDeployment.Spec.Template.Annotations = make(map[string]string)
	}
	// These annotations are required uniquely for prometheus to support both the request routing to keycloak via the envoy and the writing
	// of the Istio certs to a volume that can be accessed for scraping
	prometheusDeployment.Spec.Template.Annotations["proxy.istio.io/config"] = `{"proxyMetadata":{ "OUTPUT_CERTS": "/etc/istio-output-certs"}}`
	prometheusDeployment.Spec.Template.Annotations["sidecar.istio.io/userVolumeMount"] = `[{"name": "istio-certs-dir", "mountPath": "/etc/istio-output-certs"}]`

	resource := schema.GroupVersionResource{
		Group:    "install.verrazzano.io",
		Version:  "v1alpha1",
		Resource: "verrazzanos",
	}

	// Get the verrazzano install resource list
	unstList, err := dynamicclientset.Resource(resource).Namespace("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		zap.S().Errorf("unable to list verrazzano resources: %v", err)
		return err
	}

	// Only one verrazzano can be installed so use the first item in the returned list
	profile, found, err := unstructured.NestedString(unstList.Items[0].Object, "spec", "profile")
	if err != nil {
		zap.S().Errorf("unable to get spec.profile for verrazzano resource: %v", err)
		return err
	}

	// Determine whether keycloak has been deployed or not by checking the verrazzano resource
	var keycloakDeployed = true
	if found && profile == "managed-cluster" {
		keycloakDeployed = false // keycloak is not deployed with the managed-cluster profile
	} else {
		enabled, found, err := unstructured.NestedBool(unstList.Items[0].Object, "spec", "components", "keycloak", "enabled")
		if err != nil {
			zap.S().Errorf("unable to get spec.components.keycloak.enabled for verrazzano resource: %v", err)
			return err
		}
		if found {
			keycloakDeployed = enabled
		}
	}

	// If Keycloak isn't deployed configure Prometheus to avoid the Istio sidecar for metrics scraping.
	// This is done by adding the traffic.sidecar.istio.io/excludeOutboundIPRanges: 0.0.0.0/0 annotation.
	if !keycloakDeployed {
		prometheusDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/excludeOutboundIPRanges"] = "0.0.0.0/0"
		return nil
	}

	// Set the Istio annotation on Prometheus to exclude Keycloak HTTP Service IP address.
	// The includeOutboundIPRanges implies all others are excluded.
	// This is done by adding the traffic.sidecar.istio.io/includeOutboundIPRanges=<Keycloak IP>/32 annotation.
	resource = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "services",
	}
	unst, err := dynamicclientset.Resource(resource).Namespace("keycloak").Get(context.TODO(), "keycloak-http", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		} else {
			zap.S().Errorf("unable to get keycloak-http service: %v", err)
			return err
		}
	}
	clusterIP, found, err := unstructured.NestedString(unst.Object, "spec", "clusterIP")
	if err != nil {
		zap.S().Errorf("unable to get spec.clusterIP for keycloak-http service resource: %v", err)
		return err
	}

	if found {
		prometheusDeployment.Spec.Template.Annotations["traffic.sidecar.istio.io/includeOutboundIPRanges"] = fmt.Sprintf("%s/32", clusterIP)
	} else {
		msg := "service clusterIP not found for keycloak-http service resource"
		zap.S().Error(msg)
		return fmt.Errorf(msg)
	}

	return nil
}

// Creates *all* Prometheus-related deployment elements
func createPrometheusDeploymentElements(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, dynamicclientset dynamic.Interface, pvcToAdMap map[string]string) ([]*appsv1.Deployment, error) {
	var deployList []*appsv1.Deployment
	deployments, err := createPrometheusNodeDeploymentElements(vmo, dynamicclientset, pvcToAdMap)
	if err != nil {
		return nil, err
	}
	deployList = append(deployList, deployments...)
	return deployList, nil
}
