package resources

import vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"

func StatefulSetNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append(filterNodes(vmo, func(role vmcontrollerv1.NodeRole) bool {
		return role == vmcontrollerv1.MasterRole
	}), vmo.Spec.Elasticsearch.MasterNode)
}

func DeploymentNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) []vmcontrollerv1.ElasticsearchNode {
	return append(filterNodes(vmo, func(role vmcontrollerv1.NodeRole) bool {
		return role != vmcontrollerv1.MasterRole
	}), vmo.Spec.Elasticsearch.DataNode, vmo.Spec.Elasticsearch.IngestNode)
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
