// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package ingresses

import (
	"testing"

	"strings"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/stretchr/testify/assert"
)

func TestSauronNoIngress(t *testing.T) {
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{
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
	ingresses, err := New(sauron)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 0, len(ingresses), "Length of generated Ingresses")
}

func TestSauronWithIngresses(t *testing.T) {
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{
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
	ingresses, err := New(sauron)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, 5, len(ingresses), "Length of generated Ingresses")
	assert.Equal(t, 1, len(ingresses[0].Spec.TLS), "Number of TLS elements in generated Ingress")
	assert.Equal(t, 1, len(ingresses[0].Spec.TLS[0].Hosts), "Number of hosts in generated Ingress")
	assert.True(t, strings.Contains(ingresses[0].Spec.TLS[0].SecretName, "tls"), "TLS secret")
	assert.Equal(t, "basic", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-type"], "Auth type")
	assert.Equal(t, "secret", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-secret"], "Auth secret")
	assert.Equal(t, "example.com auth", ingresses[0].Annotations["nginx.ingress.kubernetes.io/auth-realm"], "Auth realm")
}

func TestSauronWithCascadingDelete(t *testing.T) {
	// With CascadingDelete
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{
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
	ingresses, err := New(sauron)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(ingresses) > 0, "Non-zero length generated ingresses")
	for _, ingress := range ingresses {
		assert.Equal(t, 1, len(ingress.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")
	}

	// Without CascadingDelete
	sauron.Spec.CascadingDelete = false
	ingresses, err = New(sauron)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, len(ingresses) > 0, "Non-zero length generated ingresses")
	for _, ingress := range ingresses {
		assert.Equal(t, 0, len(ingress.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
	}
}
