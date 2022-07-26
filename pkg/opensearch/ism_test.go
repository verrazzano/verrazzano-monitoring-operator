// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"strings"
	"testing"
)

const (
	testPolicyNotFound  = `{"error":{"root_cause":[{"type":"status_exception","reason":"Policy not found"}],"type":"status_exception","reason":"Policy not found"},"status":404}`
	testSettingsCreated = `{"acknowledged":true}`
	testSystemPolicy    = `
{
    "_id" : "verrazzano-system",
    "_seq_no" : 0,
    "_primary_term" : 1,
    "policy" : {
    "policy_id" : "verrazzano-system",
    "description" : "__vmi-managed__",
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

var testPolicyList = fmt.Sprintf(`{
	"policies": [
      %s
    ]
}`, testSystemPolicy)

func createTestPolicy(age, rolloverAge, indexPattern, minSize string, minDocCount int) *vmcontrollerv1.IndexManagementPolicy {
	return &vmcontrollerv1.IndexManagementPolicy{
		PolicyName:   "verrazzano-system",
		IndexPattern: indexPattern,
		MinIndexAge:  &age,
		Rollover: vmcontrollerv1.RolloverPolicy{
			MinIndexAge: &rolloverAge,
			MinSize:     &minSize,
			MinDocCount: &minDocCount,
		},
	}
}

func createISMVMI(age string, enabled bool) *vmcontrollerv1.VerrazzanoMonitoringInstance {
	v := &vmcontrollerv1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: vmcontrollerv1.VerrazzanoMonitoringInstanceSpec{
			Elasticsearch: vmcontrollerv1.Elasticsearch{
				Enabled: enabled,
				Policies: []vmcontrollerv1.IndexManagementPolicy{
					*createTestPolicy(age, age, "*", "1gb", 1),
				},
			},
		},
	}

	return v
}

// TestConfigureIndexManagementPluginISMDisabled Tests that ISM configuration when disabled
// GIVEN a default VMI instance
// WHEN I call Configure
// THEN the ISM configuration does nothing because it is disabled
func TestConfigureIndexManagementPluginISMDisabled(t *testing.T) {
	o := NewOSClient()
	assert.NoError(t, <-o.ConfigureISM(&vmcontrollerv1.VerrazzanoMonitoringInstance{}))
}

// TestConfigureIndexManagementPluginHappyPath Tests configuration of the ISM plugin
// GIVEN a VMI instance with an ISM Policy
// WHEN I call Configure
// THEN the ISM configuration is created in OpenSearch
func TestConfigureIndexManagementPluginHappyPath(t *testing.T) {
	o := NewOSClient()
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
		switch request.Method {
		case "GET":
			if strings.Contains(request.URL.Path, "verrazzano-system") {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader(testPolicyNotFound)),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(testPolicyList)),
			}, nil

		case "PUT":
			if strings.HasSuffix(request.URL.Path, "/_settings") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(testSettingsCreated)),
				}, nil
			}
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
	vmi := createISMVMI("1d", true)
	ch := o.ConfigureISM(vmi)
	assert.NoError(t, <-ch)
}

// TestGetPolicyByName Tests retrieving ISM policies by name
// GIVEN an OpenSearch instance
// WHEN I call getPolicyByName
// THEN the specified policy should be returned, if it exists
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

	o := NewOSClient()
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
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
			policy, err := o.getPolicyByName("http://localhost:9200/" + tt.policyName)
			assert.NoError(t, err)
			assert.Equal(t, tt.status, *policy.Status)
			if tt.status == http.StatusOK {
				assert.Equal(t, 0, *policy.SequenceNumber)
				assert.Equal(t, 1, *policy.PrimaryTerm)
			}
		})
	}
}

// TestPutUpdatedPolicy_PolicyExists Tests updating a policy in place
// GIVEN a policy that already exists in the server
// WHEN I call putUpdatedPolicy
// THEN the ISM policy should be updated in place IFF there are changes to the policy
func TestPutUpdatedPolicy_PolicyExists(t *testing.T) {
	httpFunc := func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(testSystemPolicy)),
		}, nil
	}

	var tests = []struct {
		name          string
		age           string
		httpFunc      func(request *http.Request) (*http.Response, error)
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
			func(request *http.Request) (*http.Response, error) {
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
		status := http.StatusOK
		existingPolicy.Status = &status
		o := NewOSClient()
		o.DoHTTP = tt.httpFunc
		t.Run(tt.name, func(t *testing.T) {
			newPolicy := &vmcontrollerv1.IndexManagementPolicy{
				PolicyName:   "verrazzano-system",
				IndexPattern: "verrazzano-system",
				MinIndexAge:  &tt.age,
			}
			updatedPolicy, err := o.putUpdatedPolicy("http://localhost:9200", newPolicy, existingPolicy)
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
	}
}

// TestPolicyNeedsUpdate Tests that the ISM policy will only be updated when it changes
// GIVEN a new ISM policy and the existing ISM policy
// WHEN I call policyNeedsUpdate
// THEN true is only returned if the new ISM policy has changed
func TestPolicyNeedsUpdate(t *testing.T) {
	basePolicy := createTestPolicy("7d", "1d", "verrazzano-system", "10gb", 1000)
	policyExtraState := toISMPolicy(basePolicy)
	policyExtraState.Policy.States = append(policyExtraState.Policy.States, PolicyState{
		Name:        "warm",
		Actions:     []map[string]interface{}{},
		Transitions: []PolicyTransition{},
	})
	var tests = []struct {
		name        string
		p1          *vmcontrollerv1.IndexManagementPolicy
		p2          *ISMPolicy
		needsUpdate bool
	}{
		{
			"no update when equal",
			basePolicy,
			toISMPolicy(basePolicy),
			false,
		},
		{
			"needs update when age changed",
			basePolicy,
			toISMPolicy(createTestPolicy("14d", "1d", "verrazzano-system", "10gb", 1000)),
			true,
		},
		{
			"needs update when rollover age changed",
			basePolicy,
			toISMPolicy(createTestPolicy("7d", "2d", "verrazzano-system", "10gb", 1000)),
			true,
		},
		{
			"needs update when index pattern changed",
			basePolicy,
			toISMPolicy(createTestPolicy("7d", "1d", "verrazzano-system-*", "10gb", 1000)),
			true,
		},
		{
			"needs update when min size changed",
			basePolicy,
			toISMPolicy(createTestPolicy("7d", "1d", "verrazzano-system", "20gb", 1000)),
			true,
		},
		{
			"needs update when min doc count changed",
			basePolicy,
			toISMPolicy(createTestPolicy("7d", "1d", "verrazzano-system", "10gb", 5000)),
			true,
		},
		{
			"needs update when states changed",
			basePolicy,
			policyExtraState,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			needsUpdate := policyNeedsUpdate(tt.p1, tt.p2)
			assert.Equal(t, tt.needsUpdate, needsUpdate)
		})
	}
}

// TestCleanupPolicies Tests cleaning up policies no longer managed by the VMI
// GIVEN a list of expected policies
// WHEN I call cleanupPolicies
// THEN then the existing policies should be queried and any non-matching members removed
func TestCleanupPolicies(t *testing.T) {
	o := NewOSClient()

	id1 := "myapp"
	id2 := "anotherapp"

	p1 := createTestPolicy("1d", "1d", id1, "1d", 1)
	p1.PolicyName = id1
	p2 := createTestPolicy("1d", "1d", id2, "1d", 1)
	p2.PolicyName = id2
	expectedPolicies := []vmcontrollerv1.IndexManagementPolicy{
		*p1,
	}

	p1ISM := toISMPolicy(p1)
	p1ISM.ID = &id1
	p2ISM := toISMPolicy(p2)
	p2ISM.ID = &id2
	existingPolicies := &PolicyList{
		Policies: []ISMPolicy{
			*p1ISM,
			*p2ISM,
		},
	}
	existingPolicyJSON, err := json.Marshal(existingPolicies)
	assert.NoError(t, err)

	var getCalls, deleteCalls int
	o.DoHTTP = func(request *http.Request) (*http.Response, error) {
		switch request.Method {
		case "GET":
			getCalls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(existingPolicyJSON)),
			}, nil
		default:
			deleteCalls++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("")),
			}, nil
		}
	}

	err = o.cleanupPolicies("http://localhost:9200", expectedPolicies)
	assert.NoError(t, err)
	assert.Equal(t, 1, getCalls)
	assert.Equal(t, 1, deleteCalls)
}

// TestIsEligibleForDeletion Tests whether a policy is eligible for deletion or not
// GIVEN a policy and the expected policy set
// WHEN I call isEligibleForDeletion
// THEN only managed policies that are not expected are eligible for deletion
func TestIsEligibleForDeletion(t *testing.T) {
	id1 := "id1"
	p1 := ISMPolicy{ID: &id1, Policy: InlinePolicy{Description: vmiManagedPolicy}}
	id2 := "id2"
	p2 := ISMPolicy{ID: &id2}
	var tests = []struct {
		name     string
		p        ISMPolicy
		e        map[string]bool
		eligible bool
	}{
		{
			"eligible when policy is managed and policy isn't expected",
			p1,
			map[string]bool{},
			true,
		},
		{
			"ineligible when policy is not managed",
			p2,
			map[string]bool{
				id1: true,
			},
			false,
		},
		{
			"ineligible when policy is managed and policy is expected",
			p1,
			map[string]bool{
				id1: true,
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := isEligibleForDeletion(tt.p, tt.e)
			assert.Equal(t, tt.eligible, res)
		})
	}
}
