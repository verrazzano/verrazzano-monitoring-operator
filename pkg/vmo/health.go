// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"encoding/json"
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"net/http"
	"os"
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
	MasterHTTPEndpoint = "VMO_MASTER_HTTP_ENDPOINT"
	HealthGreen        = "green"
)

var doHttp = func(client *http.Client, request *http.Request) (*http.Response, error) {
	return client.Do(request)
}

//IsOpenSearchReady verifies the of the OpenSearch Cluster is ready to use by checking the cluster status is green,
// and that each node is running the expected version
func IsOpenSearchReady(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) (bool, error) {
	// Verify that the cluster is Green
	clusterHealth, err := getOpenSearchClusterHealth(vmo)
	if err != nil {
		return false, err
	}
	if !(clusterHealth.Status == HealthGreen) {
		return false, nil
	}

	// Verify that the nodes are running the expected version
	nodes, err := getOpenSearchNodes(vmo)
	if err != nil {
		return false, err
	}

	// If any node is not running the expected version, the cluster is not ready
	for _, node := range nodes {
		if node.Version != config.ESWaitTargetVersion {
			return false, nil
		}
	}

	return true, nil
}

func getOpenSearchNodes(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) ([]Node, error) {
	url := getOpenSearchHTTPEndpoint(vmo) + "/_nodes/settings"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := doHttp(http.DefaultClient, req)
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

func getOpenSearchClusterHealth(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) (*ClusterHealth, error) {
	url := getOpenSearchHTTPEndpoint(vmo) + "/_cluster/health"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := doHttp(http.DefaultClient, req)
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

func getOpenSearchHTTPEndpoint(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) string {
	// The master HTTP port may be overridden if necessary.
	// This can be useful in situations where the VMO does not have direct access to the cluster service,
	// such as when you are using port-forwarding.
	masterServiceEndpoint := os.Getenv(MasterHTTPEndpoint)
	if len(masterServiceEndpoint) > 0 {
		return masterServiceEndpoint
	}
	return fmt.Sprintf("http://%s-http:%d", resources.GetMetaName(vmo.Name, config.ElasticsearchMaster.Name), constants.ESHttpPort)
}
