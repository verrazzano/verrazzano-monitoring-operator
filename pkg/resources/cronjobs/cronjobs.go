// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package cronjobs

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func New(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, cronjobName string, schedule string, initContainers []corev1.Container,
	containers []corev1.Container, volumes []corev1.Volume) *batchv1beta1.CronJob {
	one := int32(1)
	return &batchv1beta1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:            cronjobName,
			Namespace:       vmo.Namespace,
			OwnerReferences: resources.GetOwnerReferences(vmo),
		},
		Spec: batchv1beta1.CronJobSpec{
			Schedule:                   schedule,
			FailedJobsHistoryLimit:     &one,
			SuccessfulJobsHistoryLimit: &one,
			JobTemplate: batchv1beta1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Volumes:        volumes,
							InitContainers: initContainers,
							Containers:     containers,
							RestartPolicy:  corev1.RestartPolicyOnFailure,
						},
					},
				},
			},
		},
	}
}
