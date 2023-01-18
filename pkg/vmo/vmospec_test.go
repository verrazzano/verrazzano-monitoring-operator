// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"testing"
)

func TestInitNode(t *testing.T) {
	var tests = []struct {
		name     string
		node     *vmcontrollerv1.ElasticsearchNode
		expected *vmcontrollerv1.ElasticsearchNode
	}{
		{
			"adds role, and role based name when name/role are empty",
			&vmcontrollerv1.ElasticsearchNode{
				Name: "",
			},
			&vmcontrollerv1.ElasticsearchNode{
				Name:  "es-master",
				Roles: []vmcontrollerv1.NodeRole{vmcontrollerv1.MasterRole},
			},
		},
		{
			"does not change name/role when they are present",
			&vmcontrollerv1.ElasticsearchNode{
				Name: "foobar",
				Roles: []vmcontrollerv1.NodeRole{
					vmcontrollerv1.DataRole,
					vmcontrollerv1.MasterRole,
				},
			},
			&vmcontrollerv1.ElasticsearchNode{
				Name: "foobar",
				Roles: []vmcontrollerv1.NodeRole{
					vmcontrollerv1.DataRole,
					vmcontrollerv1.MasterRole,
				},
			},
		},
	}

	for _, tt := range tests {
		initNode(tt.node, vmcontrollerv1.MasterRole)
		assert.EqualValues(t, tt.expected.Roles, tt.node.Roles)
		assert.Equal(t, tt.expected.Name, tt.node.Name)
	}
}

func TestInitializeVMOSpec(t *testing.T) {
	{
		var tests = []struct {
			name            string
			givenVmiSpec    *vmcontrollerv1.VerrazzanoMonitoringInstanceSpec
			expectedVmiSpec *vmcontrollerv1.VerrazzanoMonitoringInstanceSpec
		}{
			{
				"enabled elastic search field gets converted to opensearch",
				&vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
					Elasticsearch: vmcontrollerv1.Elasticsearch{
						Enabled: true,
						Storage: vmcontrollerv1.Storage{
							Size: "1G",
						},
					},
				},
				&vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
					Opensearch: vmcontrollerv1.Opensearch{
						Enabled: true,
						Storage: vmcontrollerv1.Storage{
							Size: "1G",
						},
					},
				},
			},
			{
				"both elastic search and opensearch are enabled",
				&vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
					Elasticsearch: vmcontrollerv1.Elasticsearch{
						Enabled: true,
						Storage: vmcontrollerv1.Storage{
							Size: "1G",
						},
					},
					Opensearch: vmcontrollerv1.Opensearch{
						Enabled: true,
						Storage: vmcontrollerv1.Storage{
							Size: "2G",
						},
					},
				},
				&vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
					Opensearch: vmcontrollerv1.Opensearch{
						Enabled: true,
						Storage: vmcontrollerv1.Storage{
							Size: "2G",
						},
					},
				},
			},
		}

		for _, tt := range tests {
			handleOpensearchConversion(tt.givenVmiSpec)
			assert.EqualValues(t, tt.expectedVmiSpec.Opensearch.Storage.Size, tt.givenVmiSpec.Opensearch.Storage.Size)
			assert.EqualValues(t, vmcontrollerv1.Elasticsearch{}, tt.givenVmiSpec.Elasticsearch)
		}
	}
}

func TestInitStorageElement(t *testing.T) {
	var tests = []struct {
		name     string
		storage  *vmcontrollerv1.Storage
		expected *vmcontrollerv1.Storage
		replicas int
	}{
		{
			"does nothing when no storage is configured",
			&vmcontrollerv1.Storage{},
			&vmcontrollerv1.Storage{},
			1,
		},
		{
			"adds 1 PVC when storage is configured with 1 replica",
			&vmcontrollerv1.Storage{Size: "1G"},
			&vmcontrollerv1.Storage{
				PvcNames: []string{"pvc"},
			},
			1,
		},
		{
			"adds 3 PVCs when storage is configured with 3 replicas",
			&vmcontrollerv1.Storage{Size: "1G"},
			&vmcontrollerv1.Storage{
				PvcNames: []string{"pvc", "pvc-1", "pvc-2"},
			},
			3,
		},
		{
			"adds PVC when replicas have increased",
			&vmcontrollerv1.Storage{
				Size:     "1G",
				PvcNames: []string{"pvc", "pvc-1"},
			},
			&vmcontrollerv1.Storage{
				PvcNames: []string{"pvc", "pvc-1", "pvc-2"},
			},
			3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initStorageElement(tt.storage, tt.replicas, "pvc")
			assert.EqualValues(t, tt.expected.PvcNames, tt.storage.PvcNames)
		})
	}
}
