// Copyright (c) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
)

const fakeIndicesResponse = `{
	"verrazzano-namespace-dummyapp": {
		"aliases": {}
	},
	"verrazzano-namespace-testapp": {
		"aliases": {}
	},
    "verrazzano-namespace-metallb-system": {
        "aliases": {}
    },
    "verrazzano-namespace-verrazzano-system": {
        "aliases": {}
    },
    "verrazzano-systemd-journal": {
        "aliases": {}
    },
    "verrazzano-namespace-keycloak": {
        "aliases": {}
    },
    "verrazzano-namespace-local-path-storage": {
        "aliases": {}
    },
    "verrazzano-namespace-istio-system": {
        "aliases": {}
    },
    "verrazzano-namespace-ingress-nginx": {
        "aliases": {}
    },
    "verrazzano-namespace-cert-manager": {
        "aliases": {}
    },
    "verrazzano-namespace-kube-system": {
        "aliases": {}
    },
    "verrazzano-namespace-monitoring": {
        "aliases": {}
    },
    ".kibana_1": {
        "aliases": {
            ".kibana": {}
        }
    }
}`

const openSearchEP = "http://localhost:9200/"

// TestGetIndices tests that indices can be fetched from the OpenSearch server
// GIVEN an OpenSearch server with indices
// WHEN I call getIndices, getSystemIndices, and getApplicationIndices
// THEN I get back the expected indices, system indices, and application indices
func TestGetIndices(t *testing.T) {
	o := NewOSClient(statefulSetLister)
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(fakeIndicesResponse)),
		}, nil
	}
	log := vzlog.DefaultLogger()
	indices, err := o.getIndices(log, openSearchEP)
	assert.NoError(t, err)
	assert.Contains(t, indices, "verrazzano-namespace-keycloak")
	assert.Contains(t, indices, "verrazzano-namespace-istio-system")
	assert.Equal(t, 13, len(indices))

	systemIndices := getSystemIndices(log, indices)
	assert.Contains(t, systemIndices, "verrazzano-namespace-keycloak")
	assert.Contains(t, systemIndices, "verrazzano-namespace-istio-system")
	assert.Equal(t, 10, len(systemIndices))

	applicationIndices := getApplicationIndices(log, indices)
	assert.Contains(t, applicationIndices, "verrazzano-namespace-testapp")
	assert.Contains(t, applicationIndices, "verrazzano-namespace-dummyapp")
	assert.Equal(t, 2, len(applicationIndices))
}

// TestDataStreamExists Tests the expected data streams can be retrieved on an OpenSearch cluster
// GIVEN a cluster with data streams on it
// WHEN I call DataStreamExists
// THEN true is returned if the data stream is present
func TestDataStreamExists(t *testing.T) {
	a := assert.New(t)
	o := NewOSClient(statefulSetLister)
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, config.DataStreamName()) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}
	exists, err := o.DataStreamExists(openSearchEP, config.DataStreamName())
	a.NoError(err)
	a.True(exists)
	exists, err = o.DataStreamExists(openSearchEP, "unknown")
	a.NoError(err)
	a.False(exists)
}

// TestCalculateSeconds Tests formatting of OpenSearch time units to seconds
// GIVEN an aribtrary time unit string
// WHEN I call calculateSeconds
// THEN the number of seconds that the time unit string represents is returned
func TestCalculateSeconds(t *testing.T) {
	a := assert.New(t)
	_, err := calculateSeconds("ww5s")
	a.Error(err, "Error should be returned from exec")
	_, err = calculateSeconds("12y")
	a.Error(err, "should fail for 'months'")
	_, err = calculateSeconds("10M")
	a.Error(err, "should fail for 'months'")
	seconds, err := calculateSeconds("6d")
	a.NoError(err, "Should not fail for valid day unit")
	a.Equal(uint64(518400), seconds)
	seconds, err = calculateSeconds("120m")
	a.NoError(err, "Should not fail for valid minute unit")
	a.Equal(uint64(7200), seconds)
	seconds, err = calculateSeconds("5h")
	a.NoError(err, "Should not fail for valid hour unit")
	a.Equal(uint64(18000), seconds)
	seconds, err = calculateSeconds("20s")
	a.NoError(err, "Should not fail for valid second unit")
	a.Equal(uint64(20), seconds)
}

// TestReindexAndDeleteIndices Tests that indices are reindexed and deleted as expected
// GIVEN a cluster with indices to reindex
// WHEN I call reindexAndDeleteIndices
// THEN those indices are reindexed and deleted
func TestReindexAndDeleteIndices(t *testing.T) {
	systemIndices := []string{"verrazzano-namespace-verrazzano-system"}
	dummyOK := func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}
	var tests = []struct {
		name          string
		indices       []string
		httpFunc      func(req *http.Request) (*http.Response, error)
		isSystemIndex bool
		isError       bool
	}{
		{
			"should reindex system indices",
			systemIndices,
			dummyOK,
			true,
			false,
		},
		{
			"should reindex application indices",
			[]string{"verrazzano-namespace-bobs-books", "verrazzano-namespace-todo-app"},
			dummyOK,
			false,
			false,
		},
		{
			"should fail when reindex fails",
			systemIndices,
			func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("BOOM")
			},
			true,
			true,
		},
		{
			"should fail when delete not found",
			systemIndices,
			func(req *http.Request) (*http.Response, error) {
				if req.Method == "POST" {
					return dummyOK(req)
				}
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
			true,
			true,
		},
		{
			"should fail when delete fails",
			systemIndices,
			func(req *http.Request) (*http.Response, error) {
				if req.Method == "POST" {
					return dummyOK(req)
				}
				return nil, errors.New("BOOM")
			},
			true,
			true,
		},
		{
			"should fail when reindex 5xx",
			systemIndices,
			func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
			true,
			true,
		},
	}
	vmi := createISMVMI("1d", true)
	vmi.Spec.Elasticsearch.Policies = []vmcontrollerv1.IndexManagementPolicy{
		*createTestPolicy("1d", "1d", "verrazzano-*", "1d", 1),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewOSClient(statefulSetLister)
			o.DoHTTP = tt.httpFunc
			err := o.reindexAndDeleteIndices(vzlog.DefaultLogger(), vmi, openSearchEP, tt.indices, tt.isSystemIndex)
			if tt.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestReindexToDataStream Tests reindexing an index to a data stream
// GIVENa cluster with an index to reindex
// WHEN I call reindexToDataStream
// THEN the index is reindex to a data stream
func TestReindexToDataStream(t *testing.T) {
	o := NewOSClient(statefulSetLister)
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
		}, nil
	}
	err := o.reindexToDataStream(vzlog.DefaultLogger(), openSearchEP, "src", "dest", "1s")
	assert.NoError(t, err)
}
