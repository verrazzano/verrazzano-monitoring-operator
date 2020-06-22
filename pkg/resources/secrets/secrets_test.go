// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package secrets

import (
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"testing"
)

func TestSauronNoSecret(t *testing.T) {
	/*
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

		secrets, err := New(sauron)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, 0, len(secrets), "Length of generated Ingresses")
	*/
}

func TestSauronWithCascadingDelete(t *testing.T) {
	// With CascadingDelete
	sauron := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
		},
	}
	secret, _ := New(sauron, "secret", []byte{})
	tls, _ := NewTLS(sauron, sauron.Namespace, "secret", map[string][]byte{})
	assert.Equal(t, 1, len(secret.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")
	assert.Equal(t, 1, len(tls.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")

	// Without CascadingDelete
	sauron.Spec.CascadingDelete = false
	secret, _ = New(sauron, "secret", []byte{})
	tls, _ = NewTLS(sauron, sauron.Namespace, "secret", map[string][]byte{})
	assert.Equal(t, 0, len(secret.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
	assert.Equal(t, 0, len(tls.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
}
