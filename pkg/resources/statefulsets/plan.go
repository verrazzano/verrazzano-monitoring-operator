// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"errors"
	"github.com/verrazzano/pkg/diff"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano/pkg/log/vzlog"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	minClusterSize = 3
)

type (
	//StatefulSetPlan describes how to update the cluster statefulsets
	StatefulSetPlan struct {
		Create          []*appsv1.StatefulSet
		Update          []*appsv1.StatefulSet
		Delete          []*appsv1.StatefulSet
		Conflict        error
		ExistingCluster bool
		BounceNodes     bool
	}

	statefulSetMapping struct {
		existing           map[string]*appsv1.StatefulSet
		expected           map[string]*appsv1.StatefulSet
		isScaleDownAllowed bool
		existingSize       int32
		expectedSize       int32
	}
)

//CreatePlan creates a plan for sts that need to be created, updated, or deleted.
// if the cluster is not safe to scale down, updates/deletes will be rejected.
func CreatePlan(log vzlog.VerrazzanoLogger, existingList, expectedList []*appsv1.StatefulSet) *StatefulSetPlan {
	mapping := createStatefulSetMapping(existingList, expectedList)
	plan := &StatefulSetPlan{
		// There is no running cluster if the existing size is 0
		ExistingCluster: mapping.existingSize > 0,
		// Always bounce the master node if we are running a single node cluster
		// This permits modifications to single node clusters, which are inherently unstable
		BounceNodes: mapping.existingSize == 1,
	}
	for name, expected := range mapping.expected {
		existing, ok := mapping.existing[name]
		if !ok {
			// the STS should be created
			plan.Create = append(plan.Create, expected)
		} else if mapping.isScaleDownAllowed || !plan.ExistingCluster {
			// The cluster is in a state that allows updates, so we check if the STS has changed
			CopyFromExisting(expected, existing)
			specDiffs := diff.Diff(existing, expected)
			if specDiffs != "" || *existing.Spec.Replicas != *expected.Spec.Replicas {
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

	if !mapping.isScaleDownAllowed && plan.ExistingCluster {
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
	if expectedSize == 0 || (existingSize == 1 && expectedList[0].Name == existingList[0].Name) {
		// if we have a single node cluster, or the desired outcome is to scale everything down to 0,
		// scale down is allowed
		mapping.isScaleDownAllowed = true
	} else {
		mapping.isScaleDownAllowed = expectedSize >= minClusterSize &&
			expectedSize > existingSize/2
	}
	mapping.existingSize = existingSize
	mapping.expectedSize = expectedSize
	return mapping
}

//CopyFromExisting copies fields that should not be changed from existing to expected.
func CopyFromExisting(expected, existing *appsv1.StatefulSet) {
	// Changes to volume claim templates are forbidden
	expected.Spec.VolumeClaimTemplates = existing.Spec.VolumeClaimTemplates
	// Changes to selector are forbidden
	expected.Spec.Selector = existing.Spec.Selector
	resources.CopyImmutableEnvVars(expected.Spec.Template.Spec.Containers, existing.Spec.Template.Spec.Containers, config.ElasticsearchMaster.Name)
}
