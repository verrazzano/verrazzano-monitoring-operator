// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package statefulsets

import (
	"github.com/stretchr/testify/assert"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func createTestSTS(name string, replicas int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: replicas,
		},
	}
}

func TestCreatePlan(t *testing.T) {
	log := vzlog.DefaultLogger()
	var tests = []struct {
		name         string
		existingList []*appsv1.StatefulSet
		expectedList []*appsv1.StatefulSet
		plan         *StatefulSetPlan
	}{
		{
			"should delete nodes when expected is empty",
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
			},
			[]*appsv1.StatefulSet{},
			&StatefulSetPlan{
				ExistingCluster: true,
				Delete: []*appsv1.StatefulSet{
					createTestSTS("foo", 1),
				},
			},
		},
		{
			"should bounce nodes when scaling up single node cluster",
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
			},
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 3),
			},
			&StatefulSetPlan{
				ExistingCluster: true,
				BounceNodes:     true,
				Update: []*appsv1.StatefulSet{
					createTestSTS("foo", 3),
				},
			},
		},
		{
			"do nothing when expected and existing are the same",
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
				createTestSTS("bar", 2),
			},
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
				createTestSTS("bar", 2),
			},
			&StatefulSetPlan{ExistingCluster: true},
		},
		{
			"create when expected, but not existing",
			nil,
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
				createTestSTS("bar", 2),
			},
			&StatefulSetPlan{
				ExistingCluster: false,
				Create: []*appsv1.StatefulSet{
					createTestSTS("foo", 1),
					createTestSTS("bar", 2),
				},
			},
		},
		{
			"delete when no longer expected",
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
				createTestSTS("bar", 2),
			},
			nil,
			&StatefulSetPlan{
				ExistingCluster: true,
				Delete: []*appsv1.StatefulSet{
					createTestSTS("foo", 1),
					createTestSTS("bar", 2),
				},
			},
		},
		{
			"update when existing and expected are both present, and there is a change, and scaling is allowed",
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 3),
			},
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 4),
			},
			&StatefulSetPlan{
				ExistingCluster: true,
				Update: []*appsv1.StatefulSet{
					createTestSTS("foo", 4),
				},
			},
		},
		{
			"update allowed when existing cluster is down",
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 0),
			},
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 3),
			},
			&StatefulSetPlan{
				ExistingCluster: false,
				Update: []*appsv1.StatefulSet{
					createTestSTS("foo", 3),
				},
			},
		},
		{
			"don't update if the scaling would cause cluster downtime",
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 3),
			},
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 2),
			},
			&StatefulSetPlan{ExistingCluster: true},
		},
		{
			"don't delete if the scaling would cause cluster downtime",
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
				createTestSTS("bar", 2),
			},
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
			},
			&StatefulSetPlan{ExistingCluster: true},
		},
		{
			"scaling should be allowed on single node clusters",
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
			},
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
			},
			&StatefulSetPlan{ExistingCluster: true},
		},
		{
			"changing single node cluster name is not allowed",
			[]*appsv1.StatefulSet{
				createTestSTS("foo", 1),
			},
			[]*appsv1.StatefulSet{
				createTestSTS("bar", 1),
			},
			&StatefulSetPlan{
				ExistingCluster: true,
				Create: []*appsv1.StatefulSet{
					createTestSTS("bar", 1),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualPlan := CreatePlan(log, tt.existingList, tt.expectedList)
			assert.Equal(t, len(tt.plan.Create), len(actualPlan.Create))
			assert.Equal(t, len(tt.plan.Update), len(actualPlan.Update))
			assert.Equal(t, len(tt.plan.Delete), len(actualPlan.Delete))
			assert.Equal(t, tt.plan.ExistingCluster, actualPlan.ExistingCluster)
			assert.Equal(t, tt.plan.BounceNodes, actualPlan.BounceNodes)
		})
	}
}

func TestCopyFromContainers(t *testing.T) {
	existing := createTestSTS("foo", 1)
	existing.Spec = appsv1.StatefulSetSpec{
		VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
		},
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: config.ElasticsearchMaster.Name,
						Env: []corev1.EnvVar{
							{
								Name:  constants.ClusterInitialMasterNodes,
								Value: "z",
							},
						},
					},
				},
			},
		},
	}
	expected := createTestSTS("foo", 1)
	expected.Spec = appsv1.StatefulSetSpec{
		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: config.ElasticsearchMaster.Name,
						Env: []corev1.EnvVar{
							{
								Name:  "x",
								Value: "y",
							},
						},
					},
				},
			},
		},
	}

	assert.NotEqualValues(t, existing.Spec.VolumeClaimTemplates, expected.Spec.VolumeClaimTemplates)
	existingInitialClusterMasters := resources.GetEnvVar(&existing.Spec.Template.Spec.Containers[0], constants.ClusterInitialMasterNodes)
	expectedInitialClusterMasters := resources.GetEnvVar(&expected.Spec.Template.Spec.Containers[0], constants.ClusterInitialMasterNodes)
	assert.NotEqualValues(t, existingInitialClusterMasters, expectedInitialClusterMasters)
	CopyFromExisting(expected, existing)

	assert.EqualValues(t, existing.Spec.VolumeClaimTemplates, expected.Spec.VolumeClaimTemplates)
	existingInitialClusterMasters = resources.GetEnvVar(&existing.Spec.Template.Spec.Containers[0], constants.ClusterInitialMasterNodes)
	expectedInitialClusterMasters = resources.GetEnvVar(&expected.Spec.Template.Spec.Containers[0], constants.ClusterInitialMasterNodes)
	assert.EqualValues(t, existingInitialClusterMasters, expectedInitialClusterMasters)
}
