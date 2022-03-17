// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
	"testing"
)

const (
	testPolicyNotFound = `{"error":{"root_cause":[{"type":"status_exception","reason":"Policy not found"}],"type":"status_exception","reason":"Policy not found"},"status":404}`
	testSystemPolicy   = `
{
    "_id" : "verrazzano-system",
    "_seq_no" : 0,
    "_primary_term" : 1,
    "policy" : {
    "policy_id" : "verrazzano-system",
    "description" : "Verrazzano Index policy to rollover and delete system indices",
    "last_updated_time" : 1647551644420,
    "schema_version" : 12,
    "error_notification" : null,
    "default_state" : "ingest",
    "states" : [
        {
        "name" : "ingest",
        "actions" : [
            {
            "rollover" : {
                "min_index_age" : "1d"
            }
            }
        ],
        "transitions" : [
            {
            "state_name" : "delete",
            "conditions" : {
                "min_index_age" : "7d"
            }
            }
        ]
        },
        {
        "name" : "delete",
        "actions" : [
            {
            "delete" : { }
            }
        ],
        "transitions" : [ ]
        }
    ],
    "ism_template" : [
        {
        "index_patterns" : [
            "verrazzano-system"
        ],
        "priority" : 1,
        "last_updated_time" : 1647551644419
        }
    ]
    }
}
`
)

func makeISMVMI(age string, enabled bool) *vmcontrollerv1.VerrazzanoMonitoringInstance {
	v := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: true,
				IndexManagement: &vmcontrollerv1.IndexManagement{
					Enabled: &enabled,
				},
			},
		},
	}

	if len(age) > 0 {
		v.Spec.Elasticsearch.IndexManagement.MaxSystemIndexAge = &age
		v.Spec.Elasticsearch.IndexManagement.MaxApplicationIndexAge = &age
	}

	return v
}

func TestConfigureIndexManagementPluginISMDisabled(t *testing.T) {
	assert.NoError(t, <-ConfigureIndexManagementPlugin(&vmcontrollerv1.VerrazzanoMonitoringInstance{}))
}

func TestConfigureIndexManagementPluginHappyPath(t *testing.T) {
	doHTTP = func(client *http.Client, request *http.Request) (*http.Response, error) {
		switch request.Method {
		case "GET":
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader(testPolicyNotFound)),
			}, nil
		case "PUT":
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(testSystemPolicy)),
			}, nil
		default:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}
	}
	vmi := makeISMVMI("1d", true)
	ch := ConfigureIndexManagementPlugin(vmi)
	assert.NoError(t, <-ch)
	resetDoHTTP()
}

func TestIsIndexManagementEnabled(t *testing.T) {
	var tests = []struct {
		name    string
		vmi     *vmcontrollerv1.VerrazzanoMonitoringInstance
		enabled bool
	}{
		{
			"enabled when ism and elasticsearch enabled",
			makeISMVMI("", true),
			true,
		},
		{
			"disabled when ism disabled",
			makeISMVMI("", false),
			false,
		},
		{
			"disabled when elasticsearch disabled",
			&vmcontrollerv1.VerrazzanoMonitoringInstance{},
			false,
		},
		{
			"disabled when ism not specified",
			&vmcontrollerv1.VerrazzanoMonitoringInstance{
				Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
					Elasticsearch: vmcontrollerv1.Elasticsearch{
						Enabled: true,
					},
				},
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.enabled, isIndexManagementEnabled(tt.vmi))
		})
	}
}

func TestGetPolicyByName(t *testing.T) {
	var tests = []struct {
		name       string
		policyName string
		status     int
	}{
		{
			"policy is fetched when it exists",
			"verrazzano-system",
			200,
		},
	}

	doHTTP = func(client *http.Client, request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.Path, "verrazzano-system") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(testSystemPolicy)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(strings.NewReader(testPolicyNotFound)),
		}, nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := getPolicyByName("http://localhost:9200/" + tt.policyName)
			assert.NoError(t, err)
			assert.Equal(t, tt.status, policy.Status)
			if tt.status == http.StatusOK {
				assert.Equal(t, 0, policy.SequenceNumber)
				assert.Equal(t, 1, policy.PrimaryTerm)
			}
		})
	}

	resetDoHTTP()
}

func TestPutUpdatedPolicy_PolicyExists(t *testing.T) {
	httpFunc := func(client *http.Client, request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(testSystemPolicy)),
		}, nil
	}

	var tests = []struct {
		name          string
		age           string
		httpFunc      func(client *http.Client, request *http.Request) (*http.Response, error)
		policyUpdated bool
		hasError      bool
	}{
		{
			"Policy should be updated when it already exists and the index lifecycle has changed",
			"1d",
			httpFunc,
			true,
			false,
		},
		{
			"Policy should not be updated when the index lifecycle has not changed",
			"7d",
			httpFunc,
			false,
			false,
		},
		{
			"Policy should not be updated when the update call fails",
			"1d",
			func(client *http.Client, request *http.Request) (*http.Response, error) {
				return nil, errors.New("boom")
			},
			false,
			true,
		},
	}

	for _, tt := range tests {
		existingPolicy := &ISMPolicy{}
		err := json.NewDecoder(strings.NewReader(testSystemPolicy)).Decode(existingPolicy)
		assert.NoError(t, err)
		existingPolicy.Status = http.StatusOK
		doHTTP = tt.httpFunc
		t.Run(tt.name, func(t *testing.T) {
			updatedPolicy, err := putUpdatedPolicy("http://localhost:9200", &tt.age, &systemPolicyTemplate, existingPolicy)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if tt.policyUpdated {
				assert.NotNil(t, updatedPolicy)
			} else {
				assert.Nil(t, updatedPolicy)
			}
		})
		resetDoHTTP()
	}
}
