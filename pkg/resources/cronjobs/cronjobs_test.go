// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package cronjobs

import (
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func TestVMIWithCascadingDelete(t *testing.T) {
	// Without CascadingDelete
	vmi := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	vmi.Spec.CascadingDelete = true
	cronJob := New(vmi, "my-cron", "*", []corev1.Container{}, []corev1.Container{}, []corev1.Volume{})
	assert.Equal(t, 1, len(cronJob.ObjectMeta.OwnerReferences), "OwnerReferences is not set with CascadingDelete true")

	// Without CascadingDelete
	vmi.Spec.CascadingDelete = false
	cronJob = New(vmi, "my-cron", "*", []corev1.Container{}, []corev1.Container{}, []corev1.Volume{})
	assert.Equal(t, 0, len(cronJob.ObjectMeta.OwnerReferences), "OwnerReferences is set even with CascadingDelete false")
}
