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
