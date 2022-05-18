// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestGetPVCNames(t *testing.T) {
	sts := createTestSTS("foo", 2)
	sts.Spec.VolumeClaimTemplates = []corev1.PersistentVolumeClaim{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "p1",
			},
		},
	}

	pvcNames := GetPVCNames(sts)
	assert.Equal(t, 2, len(pvcNames))
	assert.Equal(t, "p1-foo-0", pvcNames[0])
	assert.Equal(t, "p1-foo-1", pvcNames[1])
}
