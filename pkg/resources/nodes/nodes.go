// Copyright (C) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package nodes

import (
	"bytes"
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"strings"
)

type NodeCount struct {
	// amount of nodes with 'master' role
	MasterNodes int32
	// amount of nodes with 'ingest' role
	IngestNodes int32
	// amount of nodes with 'data' role
	DataNodes int32
	// sum of node replicas
	// this may be greater than the sum of master, data, and ingest, since nodes may have 1-3 roles.
	Replicas int32
}

var (
	RoleMaster   = GetRoleLabel(vmcontrollerv1.MasterRole)
	RoleData     = GetRoleLabel(vmcontrollerv1.DataRole)
	RoleIngest   = GetRoleLabel(vmcontrollerv1.IngestRole)
	RoleAssigned = "true"
)

// MasterNodes returns the list of master role containing nodes in the VMI spec. These nodes will be created as statefulsets.
func MasterNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append([]vmcontrollerv1.ElasticsearchNode{vmo.Spec.Opensearch.MasterNode}, filterNodes(vmo, masterNodeMatcher)...)
}

// DataNodes returns the list of data nodes (that are not masters) in the VMI spec.
func DataNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append([]vmcontrollerv1.ElasticsearchNode{vmo.Spec.Opensearch.DataNode}, filterNodes(vmo, dataNodeMatcher)...)
}

// IngestNodes returns the list of ingest nodes in the VMI spec. These nodes will have no other role but ingest.
func IngestNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append([]vmcontrollerv1.ElasticsearchNode{vmo.Spec.Opensearch.IngestNode}, filterNodes(vmo, ingestNodeMatcher)...)
}

// GetRolesString turns a nodes role list into a role string
// roles: [master, ingest, data] => "master,ingest,data"
// we have to use a buffer because NodeRole is a type alias
func GetRolesString(node *vmcontrollerv1.ElasticsearchNode) string {
	var buf bytes.Buffer
	for idx, role := range node.Roles {
		buf.WriteString(string(role))
		if idx < len(node.Roles)-1 {
			buf.WriteString(",")
		}
	}
	return buf.String()
}

func GetRoleLabel(role vmcontrollerv1.NodeRole) string {
	return fmt.Sprintf("opensearch.%s/role-%s", constants.VMOGroup, string(role))
}

// SetNodeRoleLabels adds node role labels to an existing label map
// role labels follow the format: opensearch.verrazzano.io/role-<role name>=true
func SetNodeRoleLabels(node *vmcontrollerv1.ElasticsearchNode, labels map[string]string) {
	for _, role := range node.Roles {
		labels[GetRoleLabel(role)] = RoleAssigned
	}
}

// IsSingleNodeCluster Returns true if only a single master node is requested; single-node ES cluster
func IsSingleNodeCluster(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) bool {
	nodeCount := GetNodeCount(vmo)
	return nodeCount.MasterNodes == 1 && nodeCount.Replicas == 1
}

// GetNodeCount returns a struct containing the count of nodes of each role type, and the sum of all node replicas.
func GetNodeCount(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *NodeCount {
	nodeCount := &NodeCount{}
	for _, node := range AllNodes(vmo) {
		nodeCount.Replicas += node.Replicas
		for _, role := range node.Roles {
			switch role {
			case vmcontrollerv1.IngestRole:
				nodeCount.IngestNodes += node.Replicas
			case vmcontrollerv1.DataRole:
				nodeCount.DataNodes += node.Replicas
			default:
				nodeCount.MasterNodes += node.Replicas
			}
		}
	}
	return nodeCount
}

// InitialMasterNodes returns a comma separated list of master nodes for cluster bootstrapping
func InitialMasterNodes(vmoName string, masterNodes []vmcontrollerv1.ElasticsearchNode) string {
	var j int32
	var initialMasterNodes []string
	for _, node := range masterNodes {
		for j = 0; j < node.Replicas; j++ {
			initialMasterNodes = append(initialMasterNodes, resources.GetMetaName(vmoName, node.Name)+"-"+fmt.Sprintf("%d", j))
		}
	}
	return strings.Join(initialMasterNodes, ",")
}

// AllNodes returns a list of all nodes that need to be created
func AllNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append(vmo.Spec.Opensearch.Nodes, vmo.Spec.Opensearch.MasterNode, vmo.Spec.Opensearch.DataNode, vmo.Spec.Opensearch.IngestNode)
}

func filterNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, matcher func(node vmcontrollerv1.ElasticsearchNode) bool) []vmcontrollerv1.ElasticsearchNode {
	var nodes []vmcontrollerv1.ElasticsearchNode
	for _, node := range vmo.Spec.Opensearch.Nodes {
		if matcher(node) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func matcherFactory(excluded, matched func(role vmcontrollerv1.NodeRole) bool) func(node vmcontrollerv1.ElasticsearchNode) bool {
	return func(node vmcontrollerv1.ElasticsearchNode) bool {
		var isMatch bool
		for _, role := range node.Roles {
			if excluded(role) {
				return false
			}
			if matched(role) {
				isMatch = true
			}
		}
		return isMatch
	}
}

var (
	// matches any node with master role
	masterNodeMatcher = matcherFactory(func(role vmcontrollerv1.NodeRole) bool {
		return false
	}, func(role vmcontrollerv1.NodeRole) bool {
		return role == vmcontrollerv1.MasterRole
	})
	// matches nodes with data role, or data + ingest
	dataNodeMatcher = matcherFactory(func(role vmcontrollerv1.NodeRole) bool {
		return role == vmcontrollerv1.MasterRole
	}, func(role vmcontrollerv1.NodeRole) bool {
		return role == vmcontrollerv1.DataRole
	})
	// Matches only nodes who have ingest role, and nothing else
	ingestNodeMatcher = matcherFactory(func(role vmcontrollerv1.NodeRole) bool {
		return role == vmcontrollerv1.MasterRole || role == vmcontrollerv1.DataRole
	}, func(role vmcontrollerv1.NodeRole) bool {
		return role == vmcontrollerv1.IngestRole
	})
)
