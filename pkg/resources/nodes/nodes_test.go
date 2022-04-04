// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package nodes

import (
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"testing"
)

var testMasterNodes = []vmcontrollerv1.ElasticsearchNode{
	{
		Name:     "es-master",
		Replicas: 2,
	},
	{
		Name:     "a",
		Replicas: 3,
	},
	{
		Name:     "xyz",
		Replicas: 1,
	},
}

var testMultiNodeVMI = vmcontrollerv1.VerrazzanoMonitoringInstance{
	Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
		Elasticsearch: vmcontrollerv1.Elasticsearch{
			Nodes: []vmcontrollerv1.ElasticsearchNode{
				{
					Roles: []vmcontrollerv1.NodeRole{
						vmcontrollerv1.MasterRole,
						vmcontrollerv1.DataRole,
						vmcontrollerv1.IngestRole,
					},
					Replicas: 2,
				},
			},
			MasterNode: vmcontrollerv1.ElasticsearchNode{
				Roles: []vmcontrollerv1.NodeRole{
					vmcontrollerv1.MasterRole,
				},
				Replicas: 1,
			},
			DataNode: vmcontrollerv1.ElasticsearchNode{
				Roles: []vmcontrollerv1.NodeRole{
					vmcontrollerv1.DataRole,
				},
				Replicas: 1,
			},
			IngestNode: vmcontrollerv1.ElasticsearchNode{
				Roles: []vmcontrollerv1.NodeRole{
					vmcontrollerv1.IngestRole,
				},
				Replicas: 1,
			},
		},
	},
}

func TestInitialMasterNodes(t *testing.T) {
	nodeList := InitialMasterNodes("system", testMasterNodes)
	expected := "vmi-system-es-master-0,vmi-system-es-master-1,vmi-system-a-0,vmi-system-a-1,vmi-system-a-2,vmi-system-xyz-0"
	assert.Equal(t, expected, nodeList)
}

func TestGetRolesString(t *testing.T) {
	var tests = []struct {
		node      vmcontrollerv1.ElasticsearchNode
		nodeRoles string
	}{
		{
			vmcontrollerv1.ElasticsearchNode{
				Roles: []vmcontrollerv1.NodeRole{
					vmcontrollerv1.MasterRole,
				},
			},
			"master",
		},
		{
			vmcontrollerv1.ElasticsearchNode{
				Roles: []vmcontrollerv1.NodeRole{
					vmcontrollerv1.MasterRole,
					vmcontrollerv1.DataRole,
				},
			},
			"master,data",
		},
		{
			vmcontrollerv1.ElasticsearchNode{
				Roles: []vmcontrollerv1.NodeRole{
					vmcontrollerv1.MasterRole,
					vmcontrollerv1.DataRole,
					vmcontrollerv1.IngestRole,
				},
			},
			"master,data,ingest",
		},
		{
			vmcontrollerv1.ElasticsearchNode{
				Roles: []vmcontrollerv1.NodeRole{
					vmcontrollerv1.DataRole,
					vmcontrollerv1.IngestRole,
				},
			},
			"data,ingest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nodeRoles, func(t *testing.T) {
			assert.Equal(t, tt.nodeRoles, GetRolesString(&tt.node))
		})
	}
}

func TestIsSingleNodeCluster(t *testing.T) {
	var tests = []struct {
		name         string
		vmo          *vmcontrollerv1.VerrazzanoMonitoringInstance
		isSingleNode bool
	}{
		{
			"Disabled VMO is not single node",
			&vmcontrollerv1.VerrazzanoMonitoringInstance{},
			false,
		},
		{
			"Single master VMO is single node",
			&vmcontrollerv1.VerrazzanoMonitoringInstance{
				Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
					Elasticsearch: vmcontrollerv1.Elasticsearch{
						MasterNode: vmcontrollerv1.ElasticsearchNode{
							Roles: []vmcontrollerv1.NodeRole{
								vmcontrollerv1.MasterRole,
							},
							Replicas: 1,
						},
					},
				},
			},
			true,
		},
		{
			"Multi master VMO is not single node",
			&vmcontrollerv1.VerrazzanoMonitoringInstance{
				Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
					Elasticsearch: vmcontrollerv1.Elasticsearch{
						MasterNode: vmcontrollerv1.ElasticsearchNode{
							Roles: []vmcontrollerv1.NodeRole{
								vmcontrollerv1.MasterRole,
								vmcontrollerv1.IngestRole,
								vmcontrollerv1.DataRole,
							},
							Replicas: 3,
						},
					},
				},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isSingleNode, IsSingleNodeESCluster(tt.vmo))
		})
	}
}

func TestStatefulSetNodes(t *testing.T) {
	nodes := StatefulSetNodes(&testMultiNodeVMI)
	assert.Equal(t, 2, len(nodes))
}

func TestGetNodeRoleCount(t *testing.T) {
	nodeRoles := GetNodeCount(&testMultiNodeVMI)
	assert.EqualValues(t, 3, nodeRoles.DataNodes)
	assert.EqualValues(t, 3, nodeRoles.MasterNodes)
	assert.EqualValues(t, 3, nodeRoles.IngestNodes)
	assert.EqualValues(t, 5, nodeRoles.Replicas)

}
