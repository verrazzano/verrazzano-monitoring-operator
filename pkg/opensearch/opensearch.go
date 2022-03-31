// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"net/http"
)

type (
	OSClient struct {
		httpClient *http.Client
		DoHTTP     func(request *http.Request) (*http.Response, error)
	}
)

func NewOSClient() *OSClient {
	o := &OSClient{
		httpClient: http.DefaultClient,
	}
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
		return o.httpClient.Do(request)
	}
	return o
}

func (o *OSClient) IsOpenSearchResizable(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	if vmo.Spec.Elasticsearch.DataNode.Replicas < MinDataNodesForResize {
		return fmt.Errorf("cannot resize OpenSearch with less than %d data nodes. Scale up your cluster to at least %d data nodes", MinDataNodesForResize, MinDataNodesForResize)
	}
	return o.opensearchHealth(vmo, true)
}

//IsOpenSearchUpdated verifies the of the OpenSearch Cluster is ready to use by checking the cluster status is green,
// and that each node is running the expected version
func (o *OSClient) IsOpenSearchUpdated(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	return o.opensearchHealth(vmo, true)
}
