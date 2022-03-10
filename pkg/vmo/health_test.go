// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"errors"
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
	"testing"
)

const (
	wrongNodeVersion = `{
	"nodes": {
		"1": {
			"version": "1.2.3",
			"roles": [
				"master"
			]
		},
		"2": {
			"version": "1.2.3",
			"roles": [
				"ingest"
			]
		},
		"3": {
			"version": "1.2.3",
			"roles": [
				"data"
			]
		},
		"4": {
			"version": "1.2.0",
			"roles": [
				"data"
			]
		},
		"5": {
			"version": "1.2.3",
			"roles": [
				"data"
			]
		}
	}
}`
	healthyNodes = `{
	"nodes": {
		"1": {
			"version": "1.2.3",
			"roles": [
				"master"
			]
		},
		"2": {
			"version": "1.2.3",
			"roles": [
				"ingest"
			]
		},
		"3": {
			"version": "1.2.3",
			"roles": [
				"data"
			]
		},
		"4": {
			"version": "1.2.3",
			"roles": [
				"data"
			]
		},
		"5": {
			"version": "1.2.3",
			"roles": [
				"data"
			]
		}
	}
}`
	healthyCluster = `{
	"status": "green",
    "number_of_data_nodes": 3
}`
	unhealthyClusterStatus = `{
		"status": "yellow",
		"number_of_data_nodes": 3
}`
)

var testvmo = vmcontrollerv1.VerrazzanoMonitoringInstance{
	ObjectMeta: metav1.ObjectMeta{
		Name: "system",
	},
	Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
		Elasticsearch: vmcontrollerv1.Elasticsearch{
			DataNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 3,
			},
			IngestNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 1,
			},
			MasterNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 1,
			},
		},
	},
}

var testvmo2 = vmcontrollerv1.VerrazzanoMonitoringInstance{
	ObjectMeta: metav1.ObjectMeta{
		Name: "system",
	},
	Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
		Elasticsearch: vmcontrollerv1.Elasticsearch{
			DataNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 2,
			},
			IngestNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 1,
			},
			MasterNode: vmcontrollerv1.ElasticsearchNode{
				Replicas: 2,
			},
		},
	},
}

func mockHTTPGenerator(body1, body2 string, code1, code2 int) func(client *http.Client, request *http.Request) (*http.Response, error) {
	return func(client *http.Client, request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "_cluster/health") {
			return &http.Response{
				StatusCode: code1,
				Body:       io.NopCloser(strings.NewReader(body1)),
			}, nil
		}
		return &http.Response{
			StatusCode: code2,
			Body:       io.NopCloser(strings.NewReader(body2)),
		}, nil
	}
}

func TestIsOpenSearchHealthy(t *testing.T) {
	config.ESWaitTargetVersion = "1.2.3"
	var tests = []struct {
		name      string
		httpFunc  func(client *http.Client, request *http.Request) (*http.Response, error)
		isHealthy bool
		isError   bool
	}{
		{
			"healthy when cluster health is green and nodes are ready",
			mockHTTPGenerator(healthyCluster, healthyNodes, 200, 200),
			true,
			false,
		},
		{
			"unhealthy when cluster health is yellow",
			mockHTTPGenerator(unhealthyClusterStatus, healthyNodes, 200, 200),
			false,
			false,
		},
		{
			"unhealthy when expected node version is not all updated",
			mockHTTPGenerator(healthyNodes, wrongNodeVersion, 200, 200),
			false,
			false,
		},
		{
			"unhealthy when cluster status code is not OK",
			mockHTTPGenerator(healthyCluster, healthyNodes, 403, 200),
			false,
			true,
		},
		{
			"unhealthy when cluster is unreachable",
			func(client *http.Client, request *http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			},
			false,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doHttp = tt.httpFunc
			healthy, err := IsOpenSearchReady(&testvmo)
			assert.Equal(t, tt.isHealthy, healthy)
			if tt.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
