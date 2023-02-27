// Copyright (C) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/verrazzano/pkg/diff"
	"github.com/verrazzano/verrazzano-monitoring-operator"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	"net/http"
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
		Actions     []map[string]interface{} `json:"actions,omitempty"`
		Transitions []PolicyTransition       `json:"transitions,omitempty"`
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

	systemDefaultPolicyFileName = "vz-system-default-ISM-policy.json"
	appDefaultPolicyFileName    = "vz-application-default-ISM-policy.json"
	defaultPolicyPath           = "k8s/manifests/opensearch/"
	systemDefaultPolicy         = "vz-system"
	applicationDefaultPolicy    = "vz-application"
)

var (
	defaultISMPoliciesMap = map[string]string{systemDefaultPolicy: systemDefaultPolicyFileName, applicationDefaultPolicy: appDefaultPolicyFileName}
)

// createISMPolicy creates an ISM policy if it does not exist, else the policy will be updated.
// If the policy already exists and its spec matches the VMO policy spec, no update will be issued
func (o *OSClient) createISMPolicy(log vzlog.VerrazzanoLogger, opensearchEndpoint string, policy vmcontrollerv1.IndexManagementPolicy) (bool, error) {
	policyURL := fmt.Sprintf("%s/_plugins/_ism/policies/%s", opensearchEndpoint, policy.PolicyName)
	existingPolicy, err := o.getPolicyByName(policyURL)
	if err != nil {
		return false, err
	}
	exists, err := o.checkCustomISMPolicyExists(log, opensearchEndpoint, *existingPolicy)
	if err != nil {
		return false, err
	}
	if !exists {
		log.Debugf("Default ISM policy %v doesn't exists, creating now", policy.PolicyName)
		updatedPolicy, err := o.putUpdatedPolicy(opensearchEndpoint, policy.PolicyName, toISMPolicy(&policy), existingPolicy)
		if err != nil {
			return false, err
		}
		err = o.addPolicyToExistingIndices(opensearchEndpoint, &policy, updatedPolicy)
		if err != nil {
			return false, err
		}
	}
	return true, nil
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
	existingPolicy.Status = &resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		return existingPolicy, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(existingPolicy); err != nil {
		return nil, err
	}
	return existingPolicy, nil
}

// putUpdatedPolicy updates a policy in place, if the update is required. If no update was necessary, the returned
// ISMPolicy will be nil.
func (o *OSClient) putUpdatedPolicy(opensearchEndpoint string, policyName string, policy *ISMPolicy, existingPolicy *ISMPolicy) (*ISMPolicy, error) {
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
			policyName,
			*existingPolicy.SequenceNumber,
			*existingPolicy.PrimaryTerm,
		)
		statusCode = http.StatusOK
	case http.StatusNotFound: // The policy doesn't exist and must be updated
		url = fmt.Sprintf("%s/_plugins/_ism/policies/%s", opensearchEndpoint, policyName)
		statusCode = http.StatusCreated
	default:
		return nil, fmt.Errorf("invalid status when fetching ISM Policy %s: %d", policyName, existingPolicy.Status)
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
		return nil, fmt.Errorf("got status code %d when updating policy %s, expected %d", resp.StatusCode, policyName, statusCode)
	}
	updatedISMPolicy := &ISMPolicy{}
	err = json.NewDecoder(resp.Body).Decode(updatedISMPolicy)
	if err != nil {
		return nil, err
	}

	return updatedISMPolicy, nil
}

// addPolicyToExistingIndices updates any pre-existing cluster indices to be managed by the ISMPolicy
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
			if _, err := o.deletePolicy(opensearchEndpoint, *policy.ID); err != nil {
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

func (o *OSClient) deletePolicy(opensearchEndpoint, policyName string) (*http.Response, error) {
	url := fmt.Sprintf("%s/_plugins/_ism/policies/%s", opensearchEndpoint, policyName)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.DoHTTP(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("got status code %d when deleting policy %s", resp.StatusCode, policyName)
	}
	return resp, nil
}

// updateISMPolicyFromFile creates or updates the ISM policy from the given json file.
// If ISM policy doesn't exist, it will create new. Otherwise, it'll create one.
func (o *OSClient) updateISMPolicyFromFile(log vzlog.VerrazzanoLogger, openSearchEndpoint string, policyFileName string, policyName string) (*ISMPolicy, bool, error) {
	policy, err := getISMPolicyFromFile(policyFileName)
	if err != nil {
		return nil, false, err
	}
	exists, err := o.checkCustomISMPolicyExists(log, openSearchEndpoint, *policy)
	if err != nil {
		return nil, true, err
	}
	if !exists {
		existingPolicyURL := fmt.Sprintf("%s/_plugins/_ism/policies/%s", openSearchEndpoint, policyName)
		existingPolicy, err := o.getPolicyByName(existingPolicyURL)
		if err != nil {
			return nil, false, err
		}
		log.Debugf("creating ISM policy for index pattern %s", policy.Policy.ISMTemplate[0].IndexPatterns)
		policy, err = o.putUpdatedPolicy(openSearchEndpoint, policyName, policy, existingPolicy)
		if err != nil {
			return nil, false, err
		}
	}
	return policy, true, err
}

// createOrUpdateDefaultISMPolicy creates the default ISM policies if not exist, else the policies will be updated.
func (o *OSClient) createOrUpdateDefaultISMPolicy(log vzlog.VerrazzanoLogger, openSearchEndpoint string) ([]*ISMPolicy, bool, error) {
	var defaultPolicies []*ISMPolicy
	for policyName, policyFile := range defaultISMPoliciesMap {
		createdPolicy, status, err := o.updateISMPolicyFromFile(log, openSearchEndpoint, policyFile, policyName)
		if err != nil {
			return defaultPolicies, status, err
		}
		defaultPolicies = append(defaultPolicies, createdPolicy)
	}
	return defaultPolicies, true, nil
}

func isEligibleForDeletion(policy ISMPolicy, expectedPolicyMap map[string]bool) bool {
	return policy.Policy.Description == vmiManagedPolicy &&
		!expectedPolicyMap[*policy.ID]
}

// policyNeedsUpdate returns true if the policy document has changed
func policyNeedsUpdate(policy *ISMPolicy, existingPolicy *ISMPolicy) bool {
	newPolicyDocument := policy.Policy
	oldPolicyDocument := existingPolicy.Policy
	return newPolicyDocument.DefaultState != oldPolicyDocument.DefaultState ||
		newPolicyDocument.Description != oldPolicyDocument.Description ||
		diff.Diff(newPolicyDocument.States, oldPolicyDocument.States) != "" ||
		diff.Diff(newPolicyDocument.ISMTemplate, oldPolicyDocument.ISMTemplate) != ""
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

func serializeIndexManagementPolicy(policy *ISMPolicy) ([]byte, error) {
	return json.Marshal(policy)
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

// getISMPolicyFromFile reads the given json file and return the ISMPolicy object after unmarshalling.
func getISMPolicyFromFile(policyFileName string) (*ISMPolicy, error) {
	ismPolicyFS := verrazzanomonitoringoperator.GetEmbeddedISMPolicy()
	policyBytes, err := ismPolicyFS.ReadFile(defaultPolicyPath + policyFileName)
	if err != nil {
		return nil, err
	}
	var policy ISMPolicy
	err = json.Unmarshal(policyBytes, &policy)
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

func (o *OSClient) checkCustomISMPolicyExists(log vzlog.VerrazzanoLogger, opensearchEndpoint string, searchPolicy ISMPolicy) (bool, error) {
	log.Debugf("checking if ISM policy for index pattern %v exists ", searchPolicy.Policy.ISMTemplate[0].IndexPatterns)
	policyList, err := o.getAllPolicies(opensearchEndpoint)
	//Case when no polices exists in system
	if policyList == nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, policy := range policyList.Policies {
		if policy.Policy.ISMTemplate[0].Priority == searchPolicy.Policy.ISMTemplate[0].Priority && isItemAlreadyExists(log, policy.Policy.ISMTemplate[0].IndexPatterns, searchPolicy.Policy.ISMTemplate[0].IndexPatterns) {
			if policy.ID == searchPolicy.ID {
				log.Infof("VZ created default ISM policy for index pattern %v already exists", searchPolicy.Policy.ISMTemplate[0].IndexPatterns)
				return false, nil
			}
			log.Debugf("ISM policy for index pattern %v already exists ", searchPolicy.Policy.ISMTemplate[0].IndexPatterns)
			return true, nil
		}
	}
	return false, nil
}

func isItemAlreadyExists(log vzlog.VerrazzanoLogger, allListPolicyPatterns []string, subListPolicyPattern []string) bool {
	matched := false
	log.Debugf("searching for index pattern %s in all ISM policies %s", subListPolicyPattern, allListPolicyPatterns)
	for _, al := range allListPolicyPatterns {
		for _, sl := range subListPolicyPattern {
			if al == sl {
				matched = true
				break
			}
		}
	}
	return matched
}
