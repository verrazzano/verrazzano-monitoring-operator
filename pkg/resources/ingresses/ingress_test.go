// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package ingresses

import (
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
)

func TestVMONoIngress(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled: true,
			},
			Kibana: vmcontrollerv1.Kibana{
				Enabled: true,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
			},
		},
	}
	ingresses, err := New(vmo)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 0, len(ingresses), "Length of generated Ingresses")
}

func TestVMOWithIngresses(t *testing.T) {
	const vmiName = "test-vmi"
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			SecretName: "secret",
			URI:        "example.com",
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled: true,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
			},
		},
	}
	vmo.Name = vmiName
	ingresses, err := New(vmo)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 4, len(ingresses), "Length of generated Ingresses")
	assert.Equal(t, 1, len(ingresses[0].Spec.TLS), "Number of TLS elements in generated Ingress")
	assert.Equal(t, 1, len(ingresses[0].Spec.TLS[0].Hosts), "Number of hosts in generated Ingress")
	assert.Equal(t, "api.example.com", ingresses[0].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, "grafana.example.com", ingresses[1].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, "prometheus.example.com", ingresses[2].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, "elasticsearch.example.com", ingresses[3].Spec.TLS[0].Hosts[0], "TLS hosts")
	assert.Equal(t, vmiName+"-tls-api", ingresses[0].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, vmiName+"-tls-grafana", ingresses[1].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, vmiName+"-tls-prometheus", ingresses[2].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, vmiName+"-tls-es-ingest", ingresses[3].Spec.TLS[0].SecretName, "TLS secret")
	assert.Equal(t, "basic", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-type"], "Auth type")
	assert.Equal(t, "secret", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-secret"], "Auth secret")
	assert.Equal(t, "example.com auth", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-realm"], "Auth realm")
	assert.Equal(t, "true", ingresses[0].Annotations["nginx.ingress.kubernetes.io/service-upstream"], "Service upstream")
	assert.Equal(t, "${service_name}.${namespace}.svc.cluster.local", ingresses[0].Annotations["nginx.ingress.kubernetes.io/upstream-vhost"], "Upstream vhost")
	assert.Equal(t, "api.example.com", ingresses[0].Annotations["cert-manager.io/common-name"], "TLS cert CN")
	assert.Equal(t, "grafana.example.com", ingresses[1].Annotations["cert-manager.io/common-name"], "TLS cert CN")
	assert.Equal(t, "prometheus.example.com", ingresses[2].Annotations["cert-manager.io/common-name"], "TLS cert CN")
	assert.Equal(t, "elasticsearch.example.com", ingresses[3].Annotations["cert-manager.io/common-name"], "TLS cert CN")
}

func TestVMOWithCascadingDelete(t *testing.T) {
	// With CascadingDelete
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
			SecretName:      "secret",
			URI:             "example.com",
			Grafana: vmcontrollerv1.Grafana{
				Enabled: true,
			},
			Prometheus: vmcontrollerv1.Prometheus{
				Enabled:  true,
				Replicas: 1,
			},
			AlertManager: vmcontrollerv1.AlertManager{
				Enabled: true,
			},
			Kibana: vmcontrollerv1.Kibana{
				Enabled: true,
			},
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
			},
		},
	}
	ingresses, err := New(vmo)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(ingresses) > 0, "Non-zero length generated ingresses")
	for _, ingress := range ingresses {
		assert.Equal(t, 1, len(ingress.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")
	}

	// Without CascadingDelete
	vmo.Spec.CascadingDelete = false
	ingresses, err = New(vmo)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(ingresses) > 0, "Non-zero length generated ingresses")
	for _, ingress := range ingresses {
		assert.Equal(t, 0, len(ingress.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
	}
}
