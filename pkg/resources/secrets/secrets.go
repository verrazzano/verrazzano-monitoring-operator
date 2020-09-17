// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package secrets

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// New creates auth secret objects for a VMO resource
func New(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, secretName string, auth []byte) (*corev1.Secret, error) {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resources.GetMetaLabels(vmo),
			Name:            secretName,
			Namespace:       vmo.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmo),
		},
		Data: map[string][]byte{
			"auth": auth,
		},
	}, nil
}

// NewTLS creates TLS secret objects for a VMO resource
func NewTLS(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, secretName string, data map[string][]byte) (*corev1.Secret, error) {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resources.GetMetaLabels(vmo),
			Name:            secretName,
			Namespace:       vmo.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmo),
		},
		Data: data,
	}, nil
}
