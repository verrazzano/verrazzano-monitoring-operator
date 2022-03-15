package vmo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"net/http"
	"strings"
	"text/template"
)

type (
	ISMPolicy struct {
		ID             string `json:"_id"`
		PrimaryTerm    int    `json:"_primary_term"`
		SequenceNumber int    `json:"_seq_no"`
		Status         int    `json:"status"`
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
	if !isIndexManagementEnabled(vmi) {
		return nil
	}

	go func() {
		opensearchEndpoint := resources.GetOpenSearchHTTPEndpoint(vmi)
		if err := createOrUpdatePolicy(opensearchEndpoint, &systemPolicyTemplate, vmi.Spec.Elasticsearch.IndexManagement.MaxSystemIndexAge); err != nil {
			ch <- err
			return
		}
		if err := createOrUpdatePolicy(opensearchEndpoint, &applicationPolicyTemplate, vmi.Spec.Elasticsearch.IndexManagement.MaxApplicationIndexAge); err != nil {
			ch <- err
			return
		}
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

	payloadBytes, err := formatISMPayload(policy, maxIndexAge)
	if err != nil {
		return err
	}
	var resp *http.Response
	switch existingPolicy.Status {
	case http.StatusOK:
		resp, err = updateISMPolicy(opensearchEndpoint, policy.name, payloadBytes, existingPolicy)
	case http.StatusNotFound:
		resp, err = createNewISMPolicy(opensearchEndpoint, policy.name, payloadBytes)
	default:
		return errors.New("plugin 'ISM' is not enabled on your OpenSearch server")

	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	createdPolicy := &ISMPolicy{}
	if err := json.NewDecoder(resp.Body).Decode(createdPolicy); err != nil {
		return err
	}
	return addPolicyToExistingIndices(opensearchEndpoint, policy, createdPolicy)
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
	return existingPolicy, nil
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

func createNewISMPolicy(opensearchEndpoint, policyName string, payload *bytes.Buffer) (*http.Response, error) {
	url := fmt.Sprintf("%s/_plugins/_ism/_policies/%s", opensearchEndpoint, policyName)
	req, err := http.NewRequest("PUT", url, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	return doHTTP(http.DefaultClient, req)
}

func updateISMPolicy(opensearchEndpoint, policyName string, payload *bytes.Buffer, existingPolicy *ISMPolicy) (*http.Response, error) {
	url := fmt.Sprintf("%s/_plugins/_ism/_policies/%s?if_seq_no=%d&if_primary_term=%d",
		opensearchEndpoint,
		policyName,
		existingPolicy.SequenceNumber,
		existingPolicy.PrimaryTerm,
	)
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	return doHTTP(http.DefaultClient, req)
}

func addPolicyToExistingIndices(opensearchEndpoint string, policy *PolicyTemplate, createdPolicy *ISMPolicy) error {
	url := fmt.Sprintf("%s/_plugins/_ism/add/%s", opensearchEndpoint, policy.indexPattern)
	body := strings.NewReader(fmt.Sprintf(`{"policy_id": "%s"}`, createdPolicy.ID))
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	resp, err := doHTTP(http.DefaultClient, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// TODO: Check response status code is acceptable
	return err
}
