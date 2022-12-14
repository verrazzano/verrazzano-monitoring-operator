// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package services

import (
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	corev1 "k8s.io/api/core/v1"
	"testing"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOpenSearchServices1(t *testing.T) {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "myVMO",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Opensearch: vmcontrollerv1.Opensearch{
				IngestNode: vmcontrollerv1.OpensearchNode{Replicas: 5},
				MasterNode: vmcontrollerv1.OpensearchNode{Replicas: 4},
				DataNode:   vmcontrollerv1.OpensearchNode{Replicas: 3},
				Enabled:    true,
			},
		},
	}
	services := createOpenSearchServiceElements(vmo, false)
	assert.Equal(t, 4, len(services), "Length of generated services")
}

func TestOpenSearchDevProfileDefaultServices(t *testing.T) {
	vmo := createDevProfileOS()

	services := createOpenSearchServiceElements(vmo, false)
	assert.Equal(t, 4, len(services), "Length of generated services")

	masterService := services[0]
	masterHTTPService := services[1]
	dataService := services[2]
	ingestService := services[3]

	expectedSelector := resources.GetSpecID(vmo.Name, config.ElasticsearchMaster.Name)

	assert.Equal(t, ingestService.Spec.Selector, expectedSelector)
	assert.EqualValues(t, ingestService.Spec.Ports[0].Port, constants.OSHTTPPort)

	assert.EqualValues(t, masterService.Spec.Ports[0].Port, constants.OSTransportPort)

	assert.EqualValues(t, dataService.Spec.Ports[0].Port, constants.OSHTTPPort)
	assert.Equal(t, dataService.Spec.Ports[0].TargetPort, intstr.FromInt(constants.OSHTTPPort))
	assert.Equal(t, dataService.Spec.Selector, expectedSelector)

	assert.EqualValues(t, constants.OSHTTPPort, masterHTTPService.Spec.Ports[0].Port)
	assert.EqualValues(t, intstr.FromInt(constants.OSHTTPPort), masterHTTPService.Spec.Ports[0].TargetPort)
}

func TestCreateOpenSearchServicesWithNodeRoles(t *testing.T) {
	vmo := createDevProfileOS()
	services := createOpenSearchServiceElements(vmo, true)
	assert.Equal(t, 4, len(services))
	assert.EqualValues(t, map[string]string{nodes.RoleMaster: nodes.RoleAssigned}, services[0].Spec.Selector)
	assert.EqualValues(t, map[string]string{nodes.RoleMaster: nodes.RoleAssigned}, services[1].Spec.Selector)
	assert.EqualValues(t, map[string]string{nodes.RoleData: nodes.RoleAssigned}, services[2].Spec.Selector)
	assert.EqualValues(t, map[string]string{nodes.RoleIngest: nodes.RoleAssigned}, services[3].Spec.Selector)
}

func createDevProfileOS() *vmcontrollerv1.VerrazzanoMonitoringInstance {
	vmo := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: v1.ObjectMeta{
			Name: "myDevVMO",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Opensearch: vmcontrollerv1.Opensearch{
				Enabled: true,
				Storage: vmcontrollerv1.Storage{Size: ""},
				MasterNode: vmcontrollerv1.OpensearchNode{
					Replicas: 1,
					Roles: []vmcontrollerv1.NodeRole{
						vmcontrollerv1.MasterRole,
						vmcontrollerv1.IngestRole,
						vmcontrollerv1.DataRole,
					},
				},
			},
		},
	}
	return vmo
}

func TestOpenSearchPodSelector(t *testing.T) {
	selector := OpenSearchPodSelector("system")
	expected := "app in (system-es-master, system-es-data, system-os-ingest)"
	assert.Equal(t, expected, selector)
}

func createTestPod(labels map[string]string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Labels: labels,
		},
	}
}

func TestUseNodeRoleSelectors(t *testing.T) {

	var tests = []struct {
		name                string
		pods                *corev1.PodList
		useNodeRoleSelector bool
	}{
		{
			"use selector if no pods present",
			&corev1.PodList{},
			true,
		},
		{
			"use selector if all pods match",
			&corev1.PodList{
				Items: []corev1.Pod{
					createTestPod(map[string]string{nodes.RoleData: nodes.RoleAssigned}),
					createTestPod(map[string]string{nodes.RoleMaster: nodes.RoleAssigned}),
					createTestPod(map[string]string{nodes.RoleIngest: nodes.RoleAssigned}),
				},
			},
			true,
		},
		{
			"use selector if all pods match using multi-role pods",
			&corev1.PodList{
				Items: []corev1.Pod{
					createTestPod(map[string]string{
						nodes.RoleMaster: nodes.RoleAssigned,
						nodes.RoleData:   nodes.RoleAssigned,
						nodes.RoleIngest: nodes.RoleAssigned,
					}),
				},
			},
			true,
		},
		{
			"don't use selector if no matching pods",
			&corev1.PodList{
				Items: []corev1.Pod{
					createTestPod(map[string]string{}),
				},
			},
			false,
		},
		{
			"don't use selector if only some pods match",
			&corev1.PodList{
				Items: []corev1.Pod{
					createTestPod(map[string]string{nodes.RoleData: nodes.RoleAssigned}),
					createTestPod(map[string]string{nodes.RoleMaster: nodes.RoleAssigned}),
					createTestPod(map[string]string{nodes.RoleIngest: nodes.RoleAssigned}),
					createTestPod(map[string]string{}),
				},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.useNodeRoleSelector, UseNodeRoleSelector(tt.pods))
		})
	}
}
