// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

//CopyFromExisting copies fields that should not be changed on the statefulset
func CopyFromExisting(current, existing *appsv1.StatefulSet) {
	// Changes to VolumeClaimTemplates are not supported without recreating the statefulset
	current.Spec.VolumeClaimTemplates = existing.Spec.VolumeClaimTemplates
	copyFromContainers(current.Spec.Template.Spec.Containers, existing.Spec.Template.Spec.Containers)
}

func copyFromContainers(current, existing []corev1.Container) {
	getContainer := func(containers []corev1.Container) (int, *corev1.Container) {
		for idx, c := range containers {
			if c.Name == config.ElasticsearchMaster.Name {
				return idx, &c
			}
		}
		return -1, nil
	}

	// Initial master nodes should not change
	idx, currentContainer := getContainer(current)
	_, existingContainer := getContainer(existing)
	if currentContainer == nil || existingContainer == nil {
		return
	}
	existingMasterNodesVar := getEnvVar(existingContainer, constants.ClusterInitialMasterNodes)
	if existingMasterNodesVar == nil {
		return
	}
	setEnvVar(currentContainer, existingMasterNodesVar)
	current[idx] = *currentContainer
}

func getEnvVar(container *corev1.Container, name string) *corev1.EnvVar {
	for _, envVar := range container.Env {
		if envVar.Name == name {
			return &envVar
		}
	}
	return nil
}

func setEnvVar(container *corev1.Container, envVar *corev1.EnvVar) {
	for idx, env := range container.Env {
		if env.Name == envVar.Name {
			container.Env[idx] = *envVar
			return
		}
	}
	container.Env = append(container.Env, *envVar)
}
