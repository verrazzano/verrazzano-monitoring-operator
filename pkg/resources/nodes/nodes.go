// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package nodes

import (
	"bytes"
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
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

//GetRolesString turns a nodes role list into a role string
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

func AddNodeRoleLabels(node *vmcontrollerv1.ElasticsearchNode, labels map[string]string) {
	for _, role := range node.Roles {
		labels["role-"+string(role)] = "true"
	}
}

// IsSingleNodeESCluster Returns true if only a single master node is requested; single-node ES cluster
func IsSingleNodeESCluster(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) bool {
	nodeCount := GetNodeCount(vmo)
	return nodeCount.MasterNodes == 1 && nodeCount.Replicas == 1
}

//GetNodeCount returns a struct containing the count of nodes of each role type, and the sum of all node replicas.
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

//InitialMasterNodes returns a comma separated list of master nodes for cluster bootstrapping
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

//AllNodes returns a list of all nodes that need to be created
func AllNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append(vmo.Spec.Elasticsearch.Nodes, vmo.Spec.Elasticsearch.MasterNode, vmo.Spec.Elasticsearch.DataNode, vmo.Spec.Elasticsearch.IngestNode)
}

//StatefulSetNodes returns a list of nodes that should be created as statefulsets
func StatefulSetNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append([]vmcontrollerv1.ElasticsearchNode{vmo.Spec.Elasticsearch.MasterNode}, filterNodes(vmo, func(role vmcontrollerv1.NodeRole) bool {
		return role == vmcontrollerv1.MasterRole
	})...)
}

func DeploymentNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append([]vmcontrollerv1.ElasticsearchNode{vmo.Spec.Elasticsearch.DataNode, vmo.Spec.Elasticsearch.IngestNode}, filterNodes(vmo, func(role vmcontrollerv1.NodeRole) bool {
		return role != vmcontrollerv1.MasterRole
	})...)
}

func IngestOnlyNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append([]vmcontrollerv1.ElasticsearchNode{vmo.Spec.Elasticsearch.IngestNode}, filterNodes(vmo, func(role vmcontrollerv1.NodeRole) bool {
		return role == vmcontrollerv1.IngestRole
	})...)
}

func filterNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, roleMatcher func(role vmcontrollerv1.NodeRole) bool) []vmcontrollerv1.ElasticsearchNode {
	isMatch := func(roles []vmcontrollerv1.NodeRole) bool {
		for _, role := range roles {
			if roleMatcher(role) {
				return true
			}
		}
		return false
	}
	var nodes []vmcontrollerv1.ElasticsearchNode
	for _, node := range vmo.Spec.Elasticsearch.Nodes {
		if isMatch(node.Roles) {
			nodes = append(nodes, node)
		}
	}
	return nodes
}
