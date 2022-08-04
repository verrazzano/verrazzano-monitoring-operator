// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"net/http"
	"reflect"
	"strings"
)

type (
	PolicyList struct {
		Policies      []ISMPolicy `json:"policies"`
		TotalPolicies int         `json:"total_policies"`
	}

	ISMPolicy struct {
		ID             *string      `json:"_id,omitempty"`
		PrimaryTerm    *int         `json:"_primary_term,omitempty"`
		SequenceNumber *int         `json:"_seq_no,omitempty"`
		Status         *int         `json:"status,omitempty"`
		Policy         InlinePolicy `json:"policy"`
	}

	InlinePolicy struct {
		DefaultState string        `json:"default_state"`
		Description  string        `json:"description"`
		States       []PolicyState `json:"states"`
		ISMTemplate  []ISMTemplate `json:"ism_template"`
	}

	ISMTemplate struct {
		IndexPatterns []string `json:"index_patterns"`
		Priority      int      `json:"priority"`
	}

	PolicyState struct {
		Name        string                   `json:"name"`
		Actions     []map[string]interface{} `json:"actions"`
		Transitions []PolicyTransition       `json:"transitions"`
	}

	PolicyTransition struct {
		StateName  string           `json:"state_name"`
		Conditions PolicyConditions `json:"conditions"`
	}

	PolicyConditions struct {
		MinIndexAge string `json:"min_index_age"`
	}
)

const (
	minIndexAgeKey = "min_index_age"

	// Default amount of time before a policy-managed index is deleted
	defaultMinIndexAge = "7d"
	// Default amount of time before a policy-managed index is rolled over
	defaultRolloverIndexAge = "1d"
	// Descriptor to identify policies as being managed by the VMI
	vmiManagedPolicy = "__vmi-managed__"
)

//createISMPolicy creates an ISM policy if it does not exist, else the policy will be updated.
// If the policy already exsts and its spec matches the VMO policy spec, no update will be issued
func (o *OSClient) createISMPolicy(opensearchEndpoint string, policy vmcontrollerv1.IndexManagementPolicy) error {
	policyURL := fmt.Sprintf("%s/_plugins/_ism/policies/%s", opensearchEndpoint, policy.PolicyName)
	existingPolicy, err := o.getPolicyByName(policyURL)
	if err != nil {
		return err
	}
	updatedPolicy, err := o.putUpdatedPolicy(opensearchEndpoint, &policy, existingPolicy)
	if err != nil {
		return err
	}
	return o.addPolicyToExistingIndices(opensearchEndpoint, &policy, updatedPolicy)
}

func (o *OSClient) getPolicyByName(policyURL string) (*ISMPolicy, error) {
	req, err := http.NewRequest("GET", policyURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.DoHTTP(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	existingPolicy := &ISMPolicy{}
	if err := json.NewDecoder(resp.Body).Decode(existingPolicy); err != nil {
		return nil, err
	}
	existingPolicy.Status = &resp.StatusCode
	return existingPolicy, nil
}

//putUpdatedPolicy updates a policy in place, if the update is required. If no update was necessary, the returned
// ISMPolicy will be nil.
func (o *OSClient) putUpdatedPolicy(opensearchEndpoint string, policy *vmcontrollerv1.IndexManagementPolicy, existingPolicy *ISMPolicy) (*ISMPolicy, error) {
	if !policyNeedsUpdate(policy, existingPolicy) {
		return nil, nil
	}

	payload, err := serializeIndexManagementPolicy(policy)
	if err != nil {
		return nil, err
	}

	var url string
	var statusCode int
	existingPolicyStatus := *existingPolicy.Status
	switch existingPolicyStatus {
	case http.StatusOK: // The policy exists and must be updated in place if it has changed
		url = fmt.Sprintf("%s/_plugins/_ism/policies/%s?if_seq_no=%d&if_primary_term=%d",
			opensearchEndpoint,
			policy.PolicyName,
			*existingPolicy.SequenceNumber,
			*existingPolicy.PrimaryTerm,
		)
		statusCode = http.StatusOK
	case http.StatusNotFound: // The policy doesn't exist and must be updated
		url = fmt.Sprintf("%s/_plugins/_ism/policies/%s", opensearchEndpoint, policy.PolicyName)
		statusCode = http.StatusCreated
	default:
		return nil, fmt.Errorf("invalid status when fetching ISM Policy %s: %d", policy.PolicyName, existingPolicy.Status)
	}
	req, err := http.NewRequest("PUT", url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Add(contentTypeHeader, applicationJSON)
	resp, err := o.DoHTTP(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != statusCode {
		return nil, fmt.Errorf("got status code %d when updating policy %s, expected %d", resp.StatusCode, policy.PolicyName, statusCode)
	}
	updatedISMPolicy := &ISMPolicy{}
	err = json.NewDecoder(resp.Body).Decode(updatedISMPolicy)
	if err != nil {
		return nil, err
	}

	return updatedISMPolicy, nil
}

//addPolicyToExistingIndices updates any pre-existing cluster indices to be managed by the ISMPolicy
func (o *OSClient) addPolicyToExistingIndices(opensearchEndpoint string, policy *vmcontrollerv1.IndexManagementPolicy, updatedPolicy *ISMPolicy) error {
	// If no policy was updated, then there is nothing to do
	if updatedPolicy == nil {
		return nil
	}
	url := fmt.Sprintf("%s/_plugins/_ism/add/%s", opensearchEndpoint, policy.IndexPattern)
	body := strings.NewReader(fmt.Sprintf(`{"policy_id": "%s"}`, *updatedPolicy.ID))
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Add(contentTypeHeader, applicationJSON)
	resp, err := o.DoHTTP(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d when updating indicies for policy %s", resp.StatusCode, policy.PolicyName)
	}
	return nil
}

func (o *OSClient) cleanupPolicies(opensearchEndpoint string, policies []vmcontrollerv1.IndexManagementPolicy) error {
	policyList, err := o.getAllPolicies(opensearchEndpoint)
	if err != nil {
		return err
	}

	expectedPolicyMap := map[string]bool{}
	for _, policy := range policies {
		expectedPolicyMap[policy.PolicyName] = true
	}

	// A policy is eligible for deletion if it is marked as VMI managed, but the VMI no longer
	// has a policy entry for it
	for _, policy := range policyList.Policies {
		if isEligibleForDeletion(policy, expectedPolicyMap) {
			if err := o.deletePolicy(opensearchEndpoint, *policy.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (o *OSClient) getAllPolicies(opensearchEndpoint string) (*PolicyList, error) {
	url := fmt.Sprintf("%s/_plugins/_ism/policies", opensearchEndpoint)
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
		return nil, fmt.Errorf("got status code %d when querying policies for cleanup", resp.StatusCode)
	}
	policies := &PolicyList{}
	if err := json.NewDecoder(resp.Body).Decode(policies); err != nil {
		return nil, err
	}
	return policies, nil
}

func (o *OSClient) deletePolicy(opensearchEndpoint, policyName string) error {
	url := fmt.Sprintf("%s/_plugins/_ism/policies/%s", opensearchEndpoint, policyName)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	resp, err := o.DoHTTP(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d when deleting policy %s", resp.StatusCode, policyName)
	}
	return nil
}

func isEligibleForDeletion(policy ISMPolicy, expectedPolicyMap map[string]bool) bool {
	return policy.Policy.Description == vmiManagedPolicy &&
		!expectedPolicyMap[*policy.ID]
}

//policyNeedsUpdate returns true if the policy document has changed
func policyNeedsUpdate(policy *vmcontrollerv1.IndexManagementPolicy, existingPolicy *ISMPolicy) bool {
	newPolicyDocument := toISMPolicy(policy).Policy
	oldPolicyDocument := existingPolicy.Policy

	return newPolicyDocument.DefaultState != oldPolicyDocument.DefaultState ||
		!reflect.DeepEqual(newPolicyDocument.States, oldPolicyDocument.States) ||
		!reflect.DeepEqual(newPolicyDocument.ISMTemplate, oldPolicyDocument.ISMTemplate)
}

func createRolloverAction(rollover *vmcontrollerv1.RolloverPolicy) map[string]interface{} {
	rolloverAction := map[string]interface{}{}
	if rollover.MinDocCount != nil {
		rolloverAction["min_doc_count"] = *rollover.MinDocCount
	}
	if rollover.MinSize != nil {
		rolloverAction["min_size"] = *rollover.MinSize
	}
	var rolloverMinIndexAge = defaultRolloverIndexAge
	if rollover.MinIndexAge != nil {
		rolloverMinIndexAge = *rollover.MinIndexAge
	}
	rolloverAction[minIndexAgeKey] = rolloverMinIndexAge
	return rolloverAction
}

func serializeIndexManagementPolicy(policy *vmcontrollerv1.IndexManagementPolicy) ([]byte, error) {
	return json.Marshal(toISMPolicy(policy))
}

func toISMPolicy(policy *vmcontrollerv1.IndexManagementPolicy) *ISMPolicy {
	rolloverAction := map[string]interface{}{
		"rollover": createRolloverAction(&policy.Rollover),
	}
	var minIndexAge = defaultMinIndexAge
	if policy.MinIndexAge != nil {
		minIndexAge = *policy.MinIndexAge
	}

	return &ISMPolicy{
		Policy: InlinePolicy{
			DefaultState: "ingest",
			Description:  vmiManagedPolicy,
			ISMTemplate: []ISMTemplate{
				{
					Priority: 1,
					IndexPatterns: []string{
						policy.IndexPattern,
					},
				},
			},
			States: []PolicyState{
				{
					Name: "ingest",
					Actions: []map[string]interface{}{
						rolloverAction,
					},
					Transitions: []PolicyTransition{
						{
							StateName: "delete",
							Conditions: PolicyConditions{
								MinIndexAge: minIndexAge,
							},
						},
					},
				},
				{
					Name: "delete",
					Actions: []map[string]interface{}{
						{
							"delete": map[string]interface{}{},
						},
					},
					Transitions: []PolicyTransition{},
				},
			},
		},
	}
}
