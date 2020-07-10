// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package secrets

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func New(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, secretName string, auth []byte) (*corev1.Secret, error) {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resources.GetMetaLabels(vmi),
			Name:            secretName,
			Namespace:       vmi.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmi),
		},
		Data: map[string][]byte{
			"auth": auth,
		},
	}, nil
}

func NewTLS(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, secretName string, data map[string][]byte) (*corev1.Secret, error) {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resources.GetMetaLabels(vmi),
			Name:            secretName,
			Namespace:       vmi.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmi),
		},
		Data: data,
	}, nil
}
