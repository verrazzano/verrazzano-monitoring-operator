// Copyright (c) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"github.com/stretchr/testify/assert"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const fakeCatIndicesOutput = `[
  {
    "index": "verrazzano-namespace-verrazzano-system"
  },
  {
    "index": "verrazzano-logstash-2022.01.23"
  },
  {
    "index": "verrazzano-namespace-kube-system"
  },
  {
    "index": "verrazzano-namespace-fleet-system"
  },
  {
    "index": "verrazzano-namespace-ingress-nginx"
  },
  {
    "index": "verrazzano-systemd-journal"
  },
  {
    "index": "verrazzano-namespace-todo-list"
  },
  {
    "index": "verrazzano-namespace-bobs-books"
  }
]`

const fakeGetTemplateOutput = `{
  "index_templates": [
    {
      "name":"verrazzano-data-stream",
      "index_template": {
        "index_patterns": [
          "verrazzano-system",
          "verrazzano-application*"
        ],
        "template": {
          "settings": {
            "index": {
              "mapping": {
                "total_fields": {
                  "limit": "2000"
                }
              },
              "refresh_interval": "5s",
              "number_of_shards": "1",
              "auto_expand_replicas": "0-1",
              "number_of_replicas": "0"
            }
          }
        }
      }
    }
  ]
}`

const testTemplateNotFound = `{"error":{"root_cause":[{"type":"status_exception","reason":"Template not found"}],"type":"status_exception","reason":"Template not found"},"status":404}`
const openSearchEP = "http://localhost:9200/"

// TestGetSystemIndices tests the getSystemIndices function.
func TestGetSystemIndices(t *testing.T) {
	assert := assert.New(t)
	// GIVEN an Elasticsearch pod
	//  WHEN getSystemIndices is called
	//  THEN a command should be executed to get the indices information
	//   AND then Verrazzano system indices should be filtered
	//   AND no error should be returned
	o := NewOSClient()
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(fakeCatIndicesOutput)),
		}, nil
	}
	indices, err := o.getSystemIndices(vzlog.DefaultLogger(), openSearchEP)
	assert.NoError(err, "Failed to get system indices")
	assert.Contains(indices, "verrazzano-systemd-journal")
	assert.Contains(indices, "verrazzano-namespace-kube-system")
	assert.NotContains(indices, "verrazzano-namespace-bobs-books")
}

// TestGetApplicationIndices tests the getApplicationIndices function.
func TestGetApplicationIndices(t *testing.T) {
	asrt := assert.New(t)

	// GIVEN an Elasticsearch pod
	//  WHEN getApplicationIndices is called
	//  THEN a command should be executed to get the indices information
	//   AND then Verrazzano application indices should be filtered
	//   AND no error should be returned
	o := NewOSClient()
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(fakeCatIndicesOutput)),
		}, nil
	}
	indices, err := o.getApplicationIndices(vzlog.DefaultLogger(), openSearchEP)
	asrt.NoError(err, "Failed to get application indices")
	asrt.Contains(indices, "verrazzano-namespace-bobs-books")
	asrt.NotContains(indices, "verrazzano-systemd-journal")
	asrt.NotContains(indices, "verrazzano-namespace-kube-system")
}

// TestVerifyDataStreamTemplateExists tests if the template exists for data streams
func TestVerifyDataStreamTemplateExists(t *testing.T) {
	asrt := assert.New(t)

	// GIVEN an Elasticsearch pod
	//  WHEN verifyDataStreamTemplateExists is called
	//  THEN a command should be executed to get the specified template information
	//   AND no error should be returned
	o := NewOSClient()
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, dataStreamTemplateName) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(fakeGetTemplateOutput)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(testTemplateNotFound)),
		}, nil
	}
	err := o.verifyDataStreamTemplateExists(vzlog.DefaultLogger(), openSearchEP,
		dataStreamTemplateName, 2*time.Second,
		1*time.Second)
	asrt.NoError(err, "Failed to verify data stream template existence")
	err = o.verifyDataStreamTemplateExists(vzlog.DefaultLogger(), openSearchEP, "test",
		2*time.Second, 1*time.Second)
	asrt.Error(err, "Error should be returned")
}

func TestFormatReindexPayload(t *testing.T) {
	asrt := assert.New(t)
	input := ReindexInput{SourceName: "verrazzano-namespace-bobs-books",
		DestinationName: "verrazzano-application-bobs-books",
		NumberOfSeconds: "60s"}
	payload, err := formatReindexPayload(input, reindexPayload)
	asrt.NoError(err, "Error not expected")
	asrt.Contains(payload, "verrazzano-namespace-bobs-books")
	asrt.Contains(payload, "verrazzano-application-bobs-books")
	asrt.Contains(payload, "60s")
}

func TestCalculateSeconds(t *testing.T) {
	asrt := assert.New(t)
	_, err := calculateSeconds("ww5s")
	asrt.Error(err, "Error should be returned from exec")
	_, err = calculateSeconds("12y")
	asrt.Error(err, "should fail for 'years'")
	_, err = calculateSeconds("10M")
	asrt.Error(err, "should fail for 'months'")
	seconds, err := calculateSeconds("6d")
	asrt.NoError(err, "Should not fail for valid day unit")
	asrt.Equal(uint64(518400), seconds)
	seconds, err = calculateSeconds("120m")
	asrt.NoError(err, "Should not fail for valid minute unit")
	asrt.Equal(uint64(7200), seconds)
	seconds, err = calculateSeconds("5h")
	asrt.NoError(err, "Should not fail for valid hour unit")
	asrt.Equal(uint64(18000), seconds)
	seconds, err = calculateSeconds("20s")
	asrt.NoError(err, "Should not fail for valid second unit")
	asrt.Equal(uint64(20), seconds)
	seconds, err = calculateSeconds("1w")
	asrt.NoError(err, "Should not fail for valid week unit")
	asrt.Equal(uint64(604800), seconds)
}
