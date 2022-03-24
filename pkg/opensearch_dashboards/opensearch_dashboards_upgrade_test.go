// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package dashboards

import (
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"strings"
	"testing"
)

const fakeGetPatternOutput = `{
  "page": 1,
  "per_page": 20,
  "total": 2,
  "saved_objects": [
    {
      "type": "index-pattern",
      "id": "0f2ede70-8e15-11ec-abc1-6bc5e972b077",
      "attributes": {
        "title": "verrazzano-namespace-bobs-books"
      },
      "references": [],
      "migrationVersion": {
        "index-pattern": "7.6.0"
      },
      "updated_at": "2022-02-15T04:09:24.182Z",
      "version": "WzQsMV0=",
      "namespaces": [
        "default"
      ],
      "score": 0
    },
    {
      "type": "index-pattern",
      "id": "1cb7fcc0-8e15-11ec-abc1-6bc5e972b077",
      "attributes": {
        "title": "verrazzano-namespace-todo*"
      },
      "references": [],
      "migrationVersion": {
        "index-pattern": "7.6.0"
      },
      "updated_at": "2022-02-15T04:09:46.892Z",
      "version": "WzksMV0=",
      "namespaces": [
        "default"
      ],
      "score": 0
    }
  ]
}`

const openSearchDashboardsEP = "http://localhost:5601/"

// TestGetPatterns tests the getPatterns function.
func TestGetPatterns(t *testing.T) {
	a := assert.New(t)

	// GIVEN an OpenSearch Dashboards pod
	//  WHEN getPatterns is called
	//  THEN a command should be executed to get the index pattern information
	//   AND then a map of index pattern id and title should be returned
	od := NewOSDashboardsClient()
	od.DoHTTP = func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(fakeGetPatternOutput)),
		}, nil
	}
	savedObjects, err := od.getPatterns(openSearchDashboardsEP, 100)
	a.NoError(err, "Failed to get patterns from OpenSearch Dashboards")
	a.Equal(2, len(savedObjects))
	a.Contains(savedObjects, SavedObject{
		ID: "0f2ede70-8e15-11ec-abc1-6bc5e972b077",
		Attributes: Attributes{
			Title: "verrazzano-namespace-bobs-books",
		},
	})
	a.Contains(savedObjects, SavedObject{
		ID: "1cb7fcc0-8e15-11ec-abc1-6bc5e972b077",
		Attributes: Attributes{
			Title: "verrazzano-namespace-todo*",
		},
	})

}

// TestConstructUpdatedPattern tests the constructUpdatedPattern function.
func TestConstructUpdatedPattern(t *testing.T) {
	asrt := assert.New(t)
	pattern := constructUpdatedPattern("verrazzano-*")
	asrt.Equal("verrazzano-*", pattern)
	pattern = constructUpdatedPattern("verrazzano-namespace-bobs-books")
	asrt.Equal("verrazzano-application-bobs-books", pattern)
	pattern = constructUpdatedPattern("verrazzano-systemd-journal")
	asrt.Equal("verrazzano-system", pattern)
	pattern = constructUpdatedPattern("verrazzano-namespace-kube-system")
	asrt.Equal("verrazzano-system", pattern)
	pattern = constructUpdatedPattern("verrazzano-namespace-todo-*")
	asrt.Equal("verrazzano-application-todo-*", pattern)
	pattern = constructUpdatedPattern("verrazzano-namespace-s*,verrazzano-namespace-bobs-books")
	asrt.Equal("verrazzano-application-s*,verrazzano-application-bobs-books", pattern)
	pattern = constructUpdatedPattern("verrazzano-namespace-k*,verrazzano-namespace-sock-shop")
	// As verrazzano-namespace-k* matches system index verrazzano-namespace-kube-system,
	// system data stream name should also be added
	asrt.Equal("verrazzano-system,verrazzano-application-k*,verrazzano-application-sock-shop", pattern)
}
