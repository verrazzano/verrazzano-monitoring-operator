// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package configmaps

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewConfig returns a Kubernetes ConfigMap containing the runner config that is volume mounted to /etc/canary-runner/config.yaml
func NewConfig(sauron *vmcontrollerv1.VerrazzanoMonitoringInstance, mapname string, data map[string]string) *corev1.ConfigMap {

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Labels:          resources.GetMetaLabels(sauron),
			Name:            mapname,
			Namespace:       sauron.Namespace,
			OwnerReferences: resources.GetOwnerReferences(sauron),
		},
		Data: data,
	}
	return configMap
}
