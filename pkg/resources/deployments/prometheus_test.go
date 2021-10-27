// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package deployments

import (
	"k8s.io/apimachinery/pkg/runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic/fake"
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
	deployments, err := New(vmo, fake.NewSimpleDynamicClient(runtime.NewScheme()), &config.OperatorConfig{}, map[string]string{}, "vmo", "changeme")
	if err != nil {
		t.Error(err)
	}
	promDeployment, err := getDeploymentByName(constants.VMOServiceNamePrefix+"my-vmo-prometheus-0", deployments)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, 3, len(promDeployment.Spec.Template.Spec.Containers), "Length of generated containers")
	assert.Equal(t, 5, len(promDeployment.Spec.Template.Spec.Volumes), "Length of generated volumes")
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
	deployments, err := New(vmo, fake.NewSimpleDynamicClient(runtime.NewScheme()), &config.OperatorConfig{}, map[string]string{}, "vmo", "changeme")
	if err != nil {
		t.Error(err)
	}
	promDeployment, err := getDeploymentByName(constants.VMOServiceNamePrefix+"my-vmo-prometheus-0", deployments)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 3, len(promDeployment.Spec.Template.Spec.Containers), "Length of generated containers")
	assert.Equal(t, 5, len(promDeployment.Spec.Template.Spec.Volumes), "Length of generated volumes")
	assert.Equal(t, 4, len(promDeployment.Spec.Template.Spec.Containers[0].VolumeMounts), "Length of generated mounts for Prometheus node")
	assert.Equal(t, 2, len(deployments), "Length of generated deployments")
	assert.Equal(t, 2, len(promDeployment.Spec.Template.Annotations))
	assert.Equal(t, "{\"proxyMetadata\":{ \"OUTPUT_CERTS\": \"/etc/istio-output-certs\"}}", promDeployment.Spec.Template.Annotations["proxy.istio.io/config"])
	assert.Equal(t, "[{\"name\": \"istio-certs-dir\", \"mountPath\": \"/etc/istio-output-certs\"}]", promDeployment.Spec.Template.Annotations["sidecar.istio.io/userVolumeMount"])
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

	deployments, err := New(vmo, fake.NewSimpleDynamicClient(runtime.NewScheme()), &config.OperatorConfig{}, map[string]string{}, "vmo", "changeme")
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
