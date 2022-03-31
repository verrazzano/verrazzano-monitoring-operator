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

const SingleNodeClusterRole = "master,ingest,data"

type NodeRoles struct {
	// amount of nodes with 'master' role
	Master int32
	// amount of nodes with 'ingest' role
	Ingest int32
	// amount of nodes with 'data' role
	Data int32
	// sum of node replicas
	// this may be greater than the sum of master, data, and ingest, since nodes may have 1-3 roles.
	NodeCount int32
}

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

// IsSingleNodeESCluster Returns true if only a single master node is requested; single-node ES cluster
func IsSingleNodeESCluster(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) bool {
	nodeCount := GetNodeRoleCount(vmo)
	return nodeCount.Master == 1 && nodeCount.NodeCount == 1
}

// IsValidMultiNodeESCluster For a valid multi-node cluster that we have more than one node and each role is represented
func IsValidMultiNodeESCluster(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) bool {
	nodeCount := GetNodeRoleCount(vmo)
	return nodeCount.NodeCount > 1 && nodeCount.Master > 0 && nodeCount.Data > 0 && nodeCount.Ingest > 0
}

func GetNodeRoleCount(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) *NodeRoles {
	replicas := &NodeRoles{}
	for _, node := range AllNodes(vmo) {
		replicas.NodeCount += node.Replicas
		for _, role := range node.Roles {
			switch role {
			case vmcontrollerv1.IngestRole:
				replicas.Ingest++
			case vmcontrollerv1.DataRole:
				replicas.Data++
			default:
				replicas.Master++
			}
		}
	}
	return replicas
}

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

func AllNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append(vmo.Spec.Elasticsearch.Nodes, vmo.Spec.Elasticsearch.MasterNode, vmo.Spec.Elasticsearch.DataNode, vmo.Spec.Elasticsearch.IngestNode)
}

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
