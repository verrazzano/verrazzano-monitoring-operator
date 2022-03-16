package vmo

import (
	"bytes"
	"encoding/json"
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"net/http"
	"strings"
	"text/template"
)

type (
	ISMPolicy struct {
		ID             string       `json:"_id"`
		PrimaryTerm    int          `json:"_primary_term"`
		SequenceNumber int          `json:"_seq_no"`
		Status         int          `json:"status"`
		Policy         InlinePolicy `json:"policy"`
	}

	InlinePolicy struct {
		States []PolicyState `json:"states"`
	}

	PolicyState struct {
		Name        string             `json:"name"`
		Transitions []PolicyTransition `json:"transitions"`
	}

	PolicyTransition struct {
		StateName  string           `json:"state_name"`
		Conditions PolicyConditions `json:"conditions"`
	}

	PolicyConditions struct {
		MinIndexAge string `json:"min_index_age"`
	}

	ReindexInput struct {
		SourceName      string
		DestinationName string
		NumberOfSeconds string
	}
	ReindexInputWithoutQuery struct {
		SourceName      string
		DestinationName string
	}

	PolicyTemplate struct {
		name         string
		indexPattern string
		payload      string
	}
)

const (
	minIndexAge            = "min_index_age"
	defaultMinIndexAge     = "7d"
	systemDataStreamName   = "verrazzano-system"
	dataStreamTemplateName = "verrazzano-data-stream"
)

const systemISMPayloadTemplate = `{
    "policy": {
        "policy_id": "system_ingest_delete",
        "description": "Verrazzano Index policy to rollover and delete system indices",
        "schema_version": 12,
        "error_notification": null,
        "default_state": "ingest",
        "states": [
            {
                "name": "ingest",
                "actions": [
                    {
                        "rollover": {
                            "min_index_age": "1d"
                        }
                    }
                ],
                "transitions": [
                    {
                        "state_name": "delete",
                        "conditions": {
                            "min_index_age": "{{ .min_index_age }}"
                        }
                    }
                ]
            },
            {
                "name": "delete",
                "actions": [
                    {
                        "delete": {}
                    }
                ],
                "transitions": []
            }
        ],
        "ism_template": {
          "index_patterns": [
            "verrazzano-system"
          ],
          "priority": 1
        }
    }
}`

const applicationISMPayloadTemplate = `{
    "policy": {
        "policy_id": "application_ingest_delete",
        "description": "Verrazzano Index policy to rollover and delete application indices",
        "schema_version": 12,
        "error_notification": null,
        "default_state": "ingest",
        "states": [
            {
                "name": "ingest",
                "actions": [
                    {
                        "rollover": {
                            "min_index_age": "1d"
                        }
                    }
                ],
                "transitions": [
                    {
                        "state_name": "delete",
                        "conditions": {
                            "min_index_age": "{{ .min_index_age }}"
                        }
                    }
                ]
            },
            {
                "name": "delete",
                "actions": [
                    {
                        "delete": {}
                    }
                ],
                "transitions": []
            }
        ],
        "ism_template": {
          "index_patterns": [
            "verrazzano-application*"
          ],
          "priority": 1
        }
    }
}`

var (
	systemPolicyTemplate = PolicyTemplate{
		name:         "verrazzano-system",
		indexPattern: "verrazzano-system",
		payload:      systemISMPayloadTemplate,
	}
	applicationPolicyTemplate = PolicyTemplate{
		name:         "verrazzano-application",
		indexPattern: "verrazzano-application*",
		payload:      applicationISMPayloadTemplate,
	}
)

//ConfigureIndexManagementPlugin sets up the ISM Policies
func ConfigureIndexManagementPlugin(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) chan error {
	ch := make(chan error)
	// configuration is done asynchronously, as this does not need to be blocking
	go func() {
		if !isIndexManagementEnabled(vmi) {
			ch <- nil
			return
		}

		opensearchEndpoint := resources.GetOpenSearchHTTPEndpoint(vmi)
		// setup the system ISM Policy
		if err := createOrUpdatePolicy(opensearchEndpoint, &systemPolicyTemplate, vmi.Spec.Elasticsearch.IndexManagement.MaxSystemIndexAge); err != nil {
			ch <- err
			return
		}

		// setup the application ISM Policy
		if err := createOrUpdatePolicy(opensearchEndpoint, &applicationPolicyTemplate, vmi.Spec.Elasticsearch.IndexManagement.MaxApplicationIndexAge); err != nil {
			ch <- err
			return
		}
		ch <- nil
	}()

	return ch
}

func isIndexManagementEnabled(vmo *vmcontrollerv1.VerrazzanoMonitoringInstance) bool {
	return vmo.Spec.Elasticsearch.Enabled &&
		vmo.Spec.Elasticsearch.IndexManagement != nil &&
		vmo.Spec.Elasticsearch.IndexManagement.Enabled != nil &&
		*vmo.Spec.Elasticsearch.IndexManagement.Enabled
}

func createOrUpdatePolicy(opensearchEndpoint string, policy *PolicyTemplate, maxIndexAge *string) error {
	policyURL := fmt.Sprintf("%s/_plugins/_ism/policies/%s", opensearchEndpoint, policy.name)
	existingPolicy, err := getPolicyByName(policyURL)
	if err != nil {
		return err
	}
	updatedPolicy, err := putUpdatedPolicy(opensearchEndpoint, maxIndexAge, policy, existingPolicy)
	if err != nil {
		return err
	}
	return addPolicyToExistingIndices(opensearchEndpoint, policy, updatedPolicy)
}

func getPolicyByName(policyURL string) (*ISMPolicy, error) {
	req, err := http.NewRequest("GET", policyURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := doHTTP(http.DefaultClient, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	existingPolicy := &ISMPolicy{}
	if err := json.NewDecoder(resp.Body).Decode(existingPolicy); err != nil {
		return nil, err
	}
	existingPolicy.Status = resp.StatusCode
	return existingPolicy, nil
}

func putUpdatedPolicy(opensearchEndpoint string, maxIndexAge *string, policy *PolicyTemplate, existingPolicy *ISMPolicy) (*ISMPolicy, error) {
	payload, err := formatISMPayload(policy, maxIndexAge)
	if err != nil {
		return nil, err
	}

	var url string
	var statusCode int
	switch existingPolicy.Status {
	case http.StatusOK: // The policy exists and must be updated in place if it has changed
		url = fmt.Sprintf("%s/_plugins/_ism/policies/%s?if_seq_no=%d&if_primary_term=%d",
			opensearchEndpoint,
			policy.name,
			existingPolicy.SequenceNumber,
			existingPolicy.PrimaryTerm,
		)
		statusCode = http.StatusOK
	case http.StatusNotFound: // The policy doesn't exist and must be updated
		url = fmt.Sprintf("%s/_plugins/_ism/policies/%s", opensearchEndpoint, policy.name)
		statusCode = http.StatusCreated
	default:
		return nil, fmt.Errorf("invalid status when fetching ISM Policy %s: %d", policy.name, existingPolicy.Status)
	}
	req, err := http.NewRequest("PUT", url, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := doHTTP(http.DefaultClient, req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != statusCode {
		return nil, fmt.Errorf("got status code %d when updating policy %s, expected %d", resp.StatusCode, policy.name, statusCode)
	}
	updatedISMPolicy := &ISMPolicy{}
	err = json.NewDecoder(resp.Body).Decode(updatedISMPolicy)
	if err != nil {
		return nil, err
	}

	return updatedISMPolicy, nil
}

func formatISMPayload(policy *PolicyTemplate, maxIndexAge *string) (*bytes.Buffer, error) {
	tmpl, err := template.New("lifecycleManagement").
		Option("missingkey=error").
		Parse(policy.payload)
	if err != nil {
		return nil, err
	}
	values := make(map[string]string)
	putOrDefault := func(value *string, key, defaultValue string) {
		if value == nil {
			values[key] = defaultValue
		} else {
			values[key] = *value
		}
	}
	putOrDefault(maxIndexAge, minIndexAge, defaultMinIndexAge)
	buffer := &bytes.Buffer{}
	if err := tmpl.Execute(buffer, values); err != nil {
		return nil, err
	}
	return buffer, nil
}

func addPolicyToExistingIndices(opensearchEndpoint string, policy *PolicyTemplate, updatedPolicy *ISMPolicy) error {
	// If no policy was updated, then there is nothing to do
	if updatedPolicy == nil {
		return nil
	}
	url := fmt.Sprintf("%s/_plugins/_ism/add/%s", opensearchEndpoint, policy.indexPattern)
	body := strings.NewReader(fmt.Sprintf(`{"policy_id": "%s"}`, updatedPolicy.ID))
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	resp, err := doHTTP(http.DefaultClient, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d when updating indicies for policy %s", resp.StatusCode, policy.name)
	}
	return nil
}
