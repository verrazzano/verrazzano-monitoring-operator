// Copyright (C) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package dashboards

import (
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
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

func TestUpdatePatterns(t *testing.T) {
	vmiOSDEnabled := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			OpensearchDashboards: vmcontrollerv1.OpensearchDashboards{
				Enabled: true,
			},
		},
	}
	var tests = []struct {
		name       string
		httpFunc   func(request *http.Request) (*http.Response, error)
		vmi        *vmcontrollerv1.VerrazzanoMonitoringInstance
		successful bool
	}{
		{
			"successful when all patterns updated without error",
			func(request *http.Request) (*http.Response, error) {
				if request.Method == "GET" {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(fakeGetPatternOutput)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
			vmiOSDEnabled,
			true,
		},
		{
			"successful when OSD is disabled",
			nil,
			&vmcontrollerv1.VerrazzanoMonitoringInstance{},
			true,
		},
		{
			"unsuccessful when get patterns fails",
			func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusServiceUnavailable,
					Body:       io.NopCloser(strings.NewReader("boom!")),
				}, nil
			},
			vmiOSDEnabled,
			false,
		},
		{
			"unsuccessful when update index fails",
			func(request *http.Request) (*http.Response, error) {
				if request.Method == "GET" {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(fakeGetPatternOutput)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("boom!")),
				}, nil
			},
			vmiOSDEnabled,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osd := NewOSDashboardsClient()
			osd.DoHTTP = tt.httpFunc
			err := osd.UpdatePatterns(vzlog.DefaultLogger(), tt.vmi)
			assert.Equal(t, tt.successful, err == nil)
		})
	}
}

func TestExecuteUpdate(t *testing.T) {
	var tests = []struct {
		name       string
		httpFunc   func(request *http.Request) (*http.Response, error)
		successful bool
	}{
		{
			"successful when PUT updated policy succeeds",
			func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
			true,
		},
		{
			"unsuccessful when PUT updated policy fails",
			func(request *http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			},
			false,
		},
		{
			"unsuccessful when PUT updated policy has HTTP error code",
			func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osd := NewOSDashboardsClient()
			osd.DoHTTP = tt.httpFunc
			err := osd.executeUpdate(vzlog.DefaultLogger(), "http://localhost:5601", "id", "original", "updated")
			assert.Equal(t, tt.successful, err == nil)
		})
	}
}

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
	savedObjects, err := od.getPatterns(openSearchDashboardsEP, 100, "")
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

func TestCreateIndexPatternPayload(t *testing.T) {
	expected := `{"attributes":{"title":"my-pattern"}}`
	actual := createIndexPatternPayload("my-pattern")
	assert.Equal(t, expected, actual)
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
	pattern = constructUpdatedPattern("verrazzano-namespace-*")
	assert.Equal(t, "verrazzano-system,verrazzano-application-*", pattern)
}

func TestIsSystemIndexMatch(t *testing.T) {
	var tests = []struct {
		pattern string
		isMatch bool
	}{
		{
			"not a system index",
			false,
		},
		{
			"verrazzano-logstash-*",
			true,
		},
		{
			"verrazzano-systemd-journal",
			true,
		},
		{
			"verrazzano-namespace-*",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			assert.Equal(t, tt.isMatch, isSystemIndexMatch(tt.pattern))
		})
	}
}

// TestGetSavedObjectMap tests getSavedObjectMap function
// GIVEN list of savedObjects
// WHEN getSavedObjectMap is called
// THEN Map is returned having SavedObject.Title as key and SavedObject as value.
func TestGetSavedObjectMap(t *testing.T) {
	type args struct {
		savedObjects []SavedObject
	}
	savedObjectList := []SavedObject{
		{
			ID: "ID1",
			Attributes: Attributes{
				Title: "test1",
			},
		},
	}
	tests := []struct {
		name string
		args args
		want map[string]*SavedObject
	}{
		{
			"TestGetSavedObjectMap when empty list is passed",
			args{
				savedObjects: []SavedObject{},
			},
			map[string]*SavedObject{},
		},
		{
			"TestGetSavedObjectMap when saved object is passed in the list",
			args{
				savedObjects: savedObjectList,
			},
			map[string]*SavedObject{savedObjectList[0].Title: &savedObjectList[0]},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getSavedObjectMap(tt.args.savedObjects); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getSavedObjectMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
