// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"errors"
	"github.com/verrazzano/pkg/diff"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

const (
	minClusterSize = 3
)

type (
	StatefulSetPlan struct {
		Create   []*appsv1.StatefulSet
		Update   []*appsv1.StatefulSet
		Delete   []*appsv1.StatefulSet
		Conflict error
	}

	statefulSetMapping struct {
		existing           map[string]*appsv1.StatefulSet
		expected           map[string]*appsv1.StatefulSet
		isScaleDownAllowed bool
	}
)

//CreatePlan creates a plan for sts that need to be created, updated, or deleted.
// if the cluster is not safe to scale down, updates/deletes will be rejected.
func CreatePlan(log vzlog.VerrazzanoLogger, existingList, expectedList []*appsv1.StatefulSet) *StatefulSetPlan {
	mapping := createStatefulSetMapping(existingList, expectedList)
	plan := &StatefulSetPlan{}

	for name, expected := range mapping.expected {
		existing, ok := mapping.existing[name]
		if !ok {
			// the STS should be created
			plan.Create = append(plan.Create, expected)
		} else if mapping.isScaleDownAllowed {
			// the STS needs to be updated
			CopyFromExisting(expected, existing)
			specDiffs := diff.Diff(expected, existing)
			if specDiffs != "" {
				log.Oncef("Statefulset %s/%s has spec differences %s", expected.Namespace, expected.Name, specDiffs)
				plan.Update = append(plan.Update, expected)
			}
		}
	}

	for name, existing := range mapping.existing {
		// the existing STS isn't found in the expected list
		if _, ok := mapping.expected[name]; !ok && mapping.isScaleDownAllowed {
			plan.Delete = append(plan.Delete, existing)
		}
	}

	if !mapping.isScaleDownAllowed {
		plan.Conflict = errors.New("skipping OpenSearch StatefulSet delete/update, cluster cannot safely lose any master nodes")
	}

	return plan
}

//createStatefulSetMapping creates a mapping of statefulset and checks if the plan would scale the cluster to an inconsistent state.
// A cluster cannot be scaled down if:
// - if has less than minClusterSize master replicas
// - the scale down would remove half or more of the master replicas
func createStatefulSetMapping(existingList, expectedList []*appsv1.StatefulSet) *statefulSetMapping {
	mapping := &statefulSetMapping{
		existing: map[string]*appsv1.StatefulSet{},
		expected: map[string]*appsv1.StatefulSet{},
	}
	var existingSize, expectedSize int32
	for _, sts := range existingList {
		existingSize += sts.Status.ReadyReplicas
		mapping.existing[sts.Name] = sts
	}
	for _, sts := range expectedList {
		expectedSize += *sts.Spec.Replicas
		mapping.expected[sts.Name] = sts
	}

	// if expected size is one, we have a single node cluster. By definition these are
	// less resilient, so updates/restarts are allowed (otherwise they would never be possible).
	if (expectedSize == 1 && existingSize == 1 && expectedList[0].Name == existingList[0].Name) || expectedSize == 0 {
		// if we have a single node cluster, or the desired outcome is to scale everything down to 0,
		// scale down is allowed
		mapping.isScaleDownAllowed = true
	} else {
		mapping.isScaleDownAllowed = existingSize >= minClusterSize &&
			expectedSize >= minClusterSize &&
			expectedSize > existingSize/2
	}

	return mapping
}

//CopyFromExisting copies fields that should not be changed from existing to expected.
func CopyFromExisting(expected, existing *appsv1.StatefulSet) {
	// Changes to VolumeClaimTemplates are not supported without recreating the statefulset
	expected.Spec.VolumeClaimTemplates = existing.Spec.VolumeClaimTemplates
	copyFromContainers(expected.Spec.Template.Spec.Containers, existing.Spec.Template.Spec.Containers)
}

func copyFromContainers(expected, existing []corev1.Container) {
	getContainer := func(containers []corev1.Container) (int, *corev1.Container) {
		for idx, c := range containers {
			if c.Name == config.ElasticsearchMaster.Name {
				return idx, &c
			}
		}
		return -1, nil
	}

	// Initial master nodes should not change
	idx, currentContainer := getContainer(expected)
	_, existingContainer := getContainer(existing)
	if currentContainer == nil || existingContainer == nil {
		return
	}
	existingMasterNodesVar := getEnvVar(existingContainer, constants.ClusterInitialMasterNodes)
	if existingMasterNodesVar == nil {
		return
	}
	setEnvVar(currentContainer, existingMasterNodesVar)
	expected[idx] = *currentContainer
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
