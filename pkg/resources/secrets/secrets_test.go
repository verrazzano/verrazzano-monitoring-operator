// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package secrets

import (
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"testing"
)

func TestVMONoSecret(t *testing.T) {
	/*
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

		secrets, err := New(vmo)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, 0, len(secrets), "Length of generated Ingresses")
	*/
}

func TestVMOWithCascadingDelete(t *testing.T) {
	// With CascadingDelete
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			CascadingDelete: true,
		},
	}
	secret, _ := New(vmo, "secret", []byte{})
	tls, _ := NewTLS(vmo, "secret", map[string][]byte{})
	assert.Equal(t, 1, len(secret.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")
	assert.Equal(t, 1, len(tls.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")

	// Without CascadingDelete
	vmo.Spec.CascadingDelete = false
	secret, _ = New(vmo, "secret", []byte{})
	tls, _ = NewTLS(vmo, "secret", map[string][]byte{})
	assert.Equal(t, 0, len(secret.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
	assert.Equal(t, 0, len(tls.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
}
