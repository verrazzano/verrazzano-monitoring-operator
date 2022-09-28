// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
)

// GetPVCNames generates the expected PVC names of a statefulset, given its replica count
// {volumeClaimTemplate name}-{sts name}-{replica ordinal}
func GetPVCNames(statefulSet *appsv1.StatefulSet) []string {
	var pvcNames []string
	var i int32
	replicas := *statefulSet.Spec.Replicas
	for _, volumeClaimTemplate := range statefulSet.Spec.VolumeClaimTemplates {
		for i = 0; i < replicas; i++ {
			pvcName := fmt.Sprintf("%s-%s-%d", volumeClaimTemplate.Name, statefulSet.Name, i)
			pvcNames = append(pvcNames, pvcName)
		}
	}
	return pvcNames
}
