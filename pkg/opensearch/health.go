// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"encoding/json"
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	nodetool "github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources/nodes"
	"net/http"
)

type (
	ClusterHealth struct {
		Status string `json:"status"`
	}

	NodeSettings struct {
		Nodes map[string]interface{} `json:"nodes"`
	}

	Node struct {
		Version string   `json:"version"`
		Roles   []string `json:"roles"`
	}
)

const (
	HealthGreen           = "green"
	MinDataNodesForResize = 2
)

func (o *OSClient) opensearchHealth(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance, checkNodeCount bool, waitForVersion bool) error {
	// If OpenSearch is not enabled in the VMI Spec
	// Return no error meaning OS is healthy
	// This is to allow uninstalling the OS cluster
	if !vmo.Spec.Opensearch.Enabled {
		return nil
	}
	// Verify that the cluster is Green
	clusterHealth, err := o.getOpenSearchClusterHealth(vmo)
	if err != nil {
		return err
	}
	if !(clusterHealth.Status == HealthGreen) {
		return fmt.Errorf("OpenSearch health is %s", clusterHealth.Status)
	}

	// Verify that the nodes are running the expected version
	nodes, err := o.getOpenSearchNodes(vmo)
	if err != nil {
		return err
	}

	if checkNodeCount {
		// Verify the node count
		expectedNodes := int(nodetool.GetNodeCount(vmo).Replicas)
		if len(nodes) < expectedNodes {
			return fmt.Errorf("Expected %d OpenSearch nodes, got %d", expectedNodes, len(nodes))
		}
	}

	if waitForVersion {
		// If any node is not running the expected version, the cluster is not ready
		for _, node := range nodes {
			if node.Version != config.ESWaitTargetVersion {
				return fmt.Errorf("Not all OpenSearch nodes are upgrade to %s version", config.ESWaitTargetVersion)
			}
		}
	}

	return nil
}

func (o *OSClient) getOpenSearchNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) ([]Node, error) {
	url := resources.GetOpenSearchHTTPEndpoint(vmo) + "/_nodes/settings"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.DoHTTP(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get node settings: %s", resp.Status)
	}

	nodeSettings := &NodeSettings{}
	if err := json.NewDecoder(resp.Body).Decode(nodeSettings); err != nil {
		return nil, err
	}

	var nodes []Node
	for nodeKey := range nodeSettings.Nodes {
		b, err := json.Marshal(nodeSettings.Nodes[nodeKey])
		if err != nil {
			return nil, err
		}

		node := &Node{}
		if err := json.Unmarshal(b, node); err != nil {
			return nil, err
		}
		nodes = append(nodes, *node)
	}

	return nodes, nil
}

func (o *OSClient) getOpenSearchClusterHealth(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) (*ClusterHealth, error) {
	url := resources.GetOpenSearchHTTPEndpoint(vmo) + "/_cluster/health"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.DoHTTP(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get cluster health: %s", resp.Status)
	}

	clusterHealth := &ClusterHealth{}
	if err := json.NewDecoder(resp.Body).Decode(clusterHealth); err != nil {
		return nil, err
	}
	return clusterHealth, nil
}
