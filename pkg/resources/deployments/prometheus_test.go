// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestPrometheusDeploymentsNoStorage(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-vmo",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled:  true,
				Replicas: 1,
			},
		},
	}
	deployments, err := New(vmo, &config.OperatorConfig{}, map[string]string{}, "vmo", "changeme")
	if err != nil {
		t.Error(err)
	}
	promDeployment, err := getDeploymentByName(constants.VMOServiceNamePrefix+"my-vmo-prometheus-0", deployments)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, 4, len(promDeployment.Spec.Template.Spec.Containers), "Length of generated containers")
	assert.Equal(t, 8, len(promDeployment.Spec.Template.Spec.Volumes), "Length of generated volumes")
	assert.Equal(t, 4, len(promDeployment.Spec.Template.Spec.Containers[0].VolumeMounts), "Length of generated mounts for Prometheus node")
	assert.Equal(t, 2, len(deployments), "Length of generated deployments")
}

func TestPrometheusDeploymentsWithStorage(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-vmo",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled:  true,
				Replicas: 1,
				Storage: vmcontrollerv1.Storage{
					Size:     "50Gi",
					PvcNames: []string{"prometheus-pvc"},
				},
			},
		},
	}
	deployments, err := New(vmo, &config.OperatorConfig{}, map[string]string{}, "vmo", "changeme")
	if err != nil {
		t.Error(err)
	}
	promDeployment, err := getDeploymentByName(constants.VMOServiceNamePrefix+"my-vmo-prometheus-0", deployments)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 4, len(promDeployment.Spec.Template.Spec.Containers), "Length of generated containers")
	assert.Equal(t, 8, len(promDeployment.Spec.Template.Spec.Volumes), "Length of generated volumes")
	assert.Equal(t, 4, len(promDeployment.Spec.Template.Spec.Containers[0].VolumeMounts), "Length of generated mounts for Prometheus node")
	assert.Equal(t, 2, len(deployments), "Length of generated deployments")
}

func TestPrometheusDeploymentElementsWithMultiplePVCs(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-vmo",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled:  true,
				Replicas: 3,
				Storage: vmcontrollerv1.Storage{
					Size:     "50Gi",
					PvcNames: []string{"prometheus-pvc", "prometheus-pvc-1", "prometheus-pvc-2"},
				},
			},
		},
	}

	deployments, err := New(vmo, &config.OperatorConfig{}, map[string]string{}, "vmo", "changeme")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 4, len(deployments), "Length of generated deployments")

	promDeployment0, err := getDeploymentByName(constants.VMOServiceNamePrefix+"my-vmo-prometheus-0", deployments)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "prometheus-pvc", promDeployment0.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName, "Associated pvc for Prometheus 0")
	promDeployment1, err := getDeploymentByName(constants.VMOServiceNamePrefix+"my-vmo-prometheus-1", deployments)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "prometheus-pvc-1", promDeployment1.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName, "Associated pvc for Prometheus 1")
	promDeployment2, err := getDeploymentByName(constants.VMOServiceNamePrefix+"my-vmo-prometheus-2", deployments)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "prometheus-pvc-2", promDeployment2.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName, "Associated pvc for Prometheus 2")
}

// TestCreatePrometheusIstioProxyContainer tests the creation of the istio-proxy container
//  WHEN createPrometheusIstioProxyContainer is called
//  THEN a container is returned with the istio-proxy container configuration
func TestCreatePrometheusIstioProxyContainer(t *testing.T) {
	container := createPrometheusIstioProxyContainer()

	assert.Equal(t, "istio-proxy", container.Name)
	assert.Equal(t, constants.DefaultImagePullPolicy, container.ImagePullPolicy)

	assert.Equal(t, 11, len(container.Args))
	assert.Equal(t, "proxy", container.Args[0])
	assert.Equal(t, "sidecar", container.Args[1])
	assert.Equal(t, "--domain", container.Args[2])
	assert.Equal(t, "$(POD_NAMESPACE).svc.cluster.local", container.Args[3])
	assert.Equal(t, "--serviceCluster", container.Args[4])
	assert.Equal(t, "istio-proxy-prometheus", container.Args[5])
	assert.Equal(t, "--proxyLogLevel=warning", container.Args[6])
	assert.Equal(t, "--proxyComponentLogLevel=misc:error", container.Args[7])
	assert.Equal(t, "--trust-domain=cluster.local", container.Args[8])
	assert.Equal(t, "--concurrency", container.Args[9])
	assert.Equal(t, "2", container.Args[10])

	assert.Equal(t, 13, len(container.Env))
	assert.Equal(t, "OUTPUT_CERTS", container.Env[0].Name)
	assert.Equal(t, "/etc/istio-certs", container.Env[0].Value)
	assert.Equal(t, "JWT_POLICY", container.Env[1].Name)
	assert.Equal(t, "third-party-jwt", container.Env[1].Value)
	assert.Equal(t, "CA_ADDR", container.Env[2].Name)
	assert.Equal(t, "istiod.istio-system.svc:15012", container.Env[2].Value)
	assert.Equal(t, "PILOT_CERT_PROVIDER", container.Env[3].Name)
	assert.Equal(t, "istiod", container.Env[3].Value)
	assert.Equal(t, "POD_NAME", container.Env[4].Name)
	assert.Equal(t, "metadata.name", container.Env[4].ValueFrom.FieldRef.FieldPath)
	assert.Equal(t, "POD_NAMESPACE", container.Env[5].Name)
	assert.Equal(t, "metadata.namespace", container.Env[5].ValueFrom.FieldRef.FieldPath)
	assert.Equal(t, "INSTANCE_IP", container.Env[6].Name)
	assert.Equal(t, "status.podIP", container.Env[6].ValueFrom.FieldRef.FieldPath)
	assert.Equal(t, "SERVICE_ACCOUNT", container.Env[7].Name)
	assert.Equal(t, "spec.serviceAccountName", container.Env[7].ValueFrom.FieldRef.FieldPath)
	assert.Equal(t, "HOST_IP", container.Env[8].Name)
	assert.Equal(t, "status.hostIP", container.Env[8].ValueFrom.FieldRef.FieldPath)
	assert.Equal(t, "ISTIO_META_POD_NAME", container.Env[9].Name)
	assert.Equal(t, "metadata.name", container.Env[9].ValueFrom.FieldRef.FieldPath)
	assert.Equal(t, "ISTIO_META_CONFIG_NAMESPACE", container.Env[10].Name)
	assert.Equal(t, "metadata.namespace", container.Env[10].ValueFrom.FieldRef.FieldPath)
	assert.Equal(t, "ISTIO_META_MESH_ID", container.Env[11].Name)
	assert.Equal(t, "cluster.local", container.Env[11].Value)
	assert.Equal(t, "ISTIO_META_CLUSTER_ID", container.Env[12].Name)
	assert.Equal(t, "Kubernetes", container.Env[12].Value)

	assert.Equal(t, 1, len(container.Ports))
	assert.Equal(t, int32(15090), container.Ports[0].ContainerPort)
	assert.Equal(t, "http-envoy-prom", container.Ports[0].Name)
	assert.Equal(t, corev1.ProtocolTCP, container.Ports[0].Protocol)

	assert.Equal(t, int32(30), container.ReadinessProbe.FailureThreshold)
	assert.Equal(t, "/healthz/ready", container.ReadinessProbe.HTTPGet.Path)
	assert.Equal(t, intstr.Int, container.ReadinessProbe.HTTPGet.Port.Type)
	assert.Equal(t, int32(15020), container.ReadinessProbe.HTTPGet.Port.IntVal)
	assert.Equal(t, corev1.URISchemeHTTP, container.ReadinessProbe.HTTPGet.Scheme)
	assert.Equal(t, int32(1), container.ReadinessProbe.InitialDelaySeconds)
	assert.Equal(t, int32(2), container.ReadinessProbe.PeriodSeconds)
	assert.Equal(t, int32(1), container.ReadinessProbe.SuccessThreshold)
	assert.Equal(t, int32(1), container.ReadinessProbe.TimeoutSeconds)

	assert.Equal(t, 4, len(container.VolumeMounts))
	assert.Equal(t, "/var/run/secrets/istio", container.VolumeMounts[0].MountPath)
	assert.Equal(t, "istiod-ca-cert", container.VolumeMounts[0].Name)
	assert.Equal(t, "/etc/istio/proxy", container.VolumeMounts[1].MountPath)
	assert.Equal(t, "istio-envoy", container.VolumeMounts[1].Name)
	assert.Equal(t, "/var/run/secrets/tokens", container.VolumeMounts[2].MountPath)
	assert.Equal(t, "istio-token", container.VolumeMounts[2].Name)
	assert.Equal(t, constants.PrometheusIstioCertsMountPath, container.VolumeMounts[3].MountPath)
	assert.Equal(t, "istio-certs", container.VolumeMounts[3].Name)
}
