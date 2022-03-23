// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"
)

var (
	// The system namespaces used in Verrazzano
	systemNamespaces = []string{"kube-system", "verrazzano-system", "istio-system", "keycloak", "metallb-system",
		"default", "cert-manager", "local-path-storage", "rancher-operator-system", "fleet-system", "ingress-nginx",
		"cattle-system", "verrazzano-install", "monitoring"}
	agePattern       = "^(?P<number>\\d+)(?P<unit>[yMwdhHms])$"
	reTimeUnit       = regexp.MustCompile(agePattern)
	secondsPerMinute = uint64(60)
	secondsPerHour   = secondsPerMinute * 60
	secondsPerDay    = secondsPerHour * 24
	secondsPerWeek   = secondsPerDay * 7
)

const (
	systemDataStreamName   = "verrazzano-system"
	dataStreamTemplateName = "verrazzano-data-stream"
)

const reindexPayload = `{
  "conflicts": "proceed",
  "source": {
    "index": "{{ .SourceName }}",
    "query": {
      "range": {
        "@timestamp": {
          "gte": "now-{{ .NumberOfSeconds }}/s",
          "lt": "now/s"
        }
      }
    }
  },
  "dest": {
    "index": "{{ .DestinationName }}",
    "op_type": "create"
  }
}`

const reindexPayloadWithoutQuery = `{
  "conflicts": "proceed",
  "source": {
    "index": "{{ .SourceName }}"
  },
  "dest": {
    "index": "{{ .DestinationName }}",
    "op_type": "create"
  }
}`

type (
	ReindexInput struct {
		SourceName      string
		DestinationName string
		NumberOfSeconds string
	}
	ReindexInputWithoutQuery struct {
		SourceName      string
		DestinationName string
	}
)

// Reindex old style indices to data streams and delete it
func (o *OSClient) MigrateIndicesToDataStreams(log vzlog.VerrazzanoLogger,
	vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, openSearchEndpoint string) error {
	log.Info("OpenSearch: Migrating Verrazzano old indices if any to data streams")
	// Make sure that the data stream template is created before re-indexing
	err := o.verifyDataStreamTemplateExists(log, openSearchEndpoint, dataStreamTemplateName, 2*time.Minute,
		15*time.Second)
	if err != nil {
		return log.ErrorfNewErr("OpenSearch: Error in verifying the existence of"+
			" data stream template %s: %v", dataStreamTemplateName, err)
	}
	// Get system indices
	systemIndices, err := o.getSystemIndices(log, openSearchEndpoint)
	if err != nil {
		return log.ErrorfNewErr("OpenSearch: Error in getting the Verrazzano system indices: %v", err)
	}

	// Reindex and delete old system indices
	err = o.reindexAndDeleteIndices(log, vmi, openSearchEndpoint, systemIndices, true)
	if err != nil {
		return log.ErrorfNewErr("OpenSearch: Error in migrating the old Verrazzano"+
			" system indices to data streams: %v", err)
	}
	// Get application indices
	appIndices, err := o.getApplicationIndices(log, openSearchEndpoint)
	if err != nil {
		return log.ErrorfNewErr("OpenSearch: Error in getting the Verrazzano application indices: %v", err)
	}
	// Reindex and delete old application indices
	err = o.reindexAndDeleteIndices(log, vmi, openSearchEndpoint, appIndices, false)
	if err != nil {
		return log.ErrorfNewErr("OpenSearch: Error in migrating the old Verrazzano"+
			" application indices to data streams: %v", err)
	}
	if len(systemIndices) > 0 || len(appIndices) > 0 {
		log.Info("OpenSearch: Migration of Verrazzano old indices to data streams completed successfully")
	} else {
		log.Info("OpenSearch: Found no old indices to migrate to data streams")
	}
	return nil
}

func (o *OSClient) verifyDataStreamTemplateExists(log vzlog.VerrazzanoLogger, openSearchEndpoint string, templateName string,
	retryDelay time.Duration, timeout time.Duration) error {
	templateURL := fmt.Sprintf("%s/_index_template/%s", openSearchEndpoint, templateName)
	log.Debugf("OpenSearch: Executing get template API %s", templateURL)
	start := time.Now()
	for {
		req, err := http.NewRequest("GET", templateURL, nil)
		if err != nil {
			return err
		}

		response, err := o.DoHTTP(req)
		if err != nil {
			return err
		}

		if response.StatusCode == http.StatusOK {
			log.Debugf("OpenSearch: Template %s exists", templateName)
			return nil
		}

		if response.StatusCode != http.StatusNotFound {
			return fmt.Errorf("got status code %d when checking for the index template %s", response.StatusCode,
				templateName)
		}

		if time.Since(start) >= timeout {
			return log.ErrorfNewErr("OpenSearch: Time out in verifying the existence of "+
				"data stream template %s", templateName)
		}
		time.Sleep(retryDelay)
	}
}

func (o *OSClient) getSystemIndices(log vzlog.VerrazzanoLogger, openSearchEndpoint string) ([]string, error) {
	var indices []string
	indices, err := o.getIndices(log, openSearchEndpoint)
	if err != nil {
		return nil, err
	}
	var systemIndices []string
	for _, index := range indices {
		if strings.HasPrefix(index, "verrazzano-namespace-") {
			for _, systemNamespace := range systemNamespaces {
				if index == "verrazzano-namespace-"+systemNamespace {
					systemIndices = append(systemIndices, index)
				}
			}
		}
		if strings.Contains(index, "verrazzano-systemd-journal") {
			systemIndices = append(systemIndices, index)
		}
		if strings.HasPrefix(index, "verrazzano-logstash-") {
			systemIndices = append(systemIndices, index)
		}
	}
	log.Debugf("OpenSearch: Found Verrazzano system indices %v", systemIndices)
	return systemIndices, nil
}

func (o *OSClient) getApplicationIndices(log vzlog.VerrazzanoLogger, openSearchEndpoint string) ([]string, error) {
	var indices []string
	indices, err := o.getIndices(log, openSearchEndpoint)
	if err != nil {
		return nil, err
	}
	var appIndices []string
	for _, index := range indices {
		systemIndex := false
		if strings.HasPrefix(index, "verrazzano-namespace-") {
			for _, systemNamespace := range systemNamespaces {
				if index == "verrazzano-namespace-"+systemNamespace {
					systemIndex = true
					break
				}
			}
			if !systemIndex {
				appIndices = append(appIndices, index)
			}
		}
	}
	log.Debugf("OpenSearch: Found Verrazzano application indices %v", appIndices)
	return appIndices, nil
}

func (o *OSClient) getIndices(log vzlog.VerrazzanoLogger, openSearchEndpoint string) ([]string, error) {
	indicesURL := fmt.Sprintf("%s/_cat/indices/verrazzano-*?format=json", openSearchEndpoint)
	log.Debugf("OpenSearch: Executing get indices API %s", indicesURL)
	req, err := http.NewRequest("GET", indicesURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := o.DoHTTP(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got status code %d when getting the indices in OpenSearch", resp.StatusCode)
	}

	log.Debugf("OpenSearch: Response body %v", resp.Body)
	var indices []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&indices)
	if err != nil {
		log.Errorf("OpenSearch: Error unmarshalling indices response body: %v", err)
		return nil, err
	}
	var indexNames []string
	for _, index := range indices {
		val, found := index["index"]
		if !found {
			log.Errorf("OpenSearch: Not able to find the name of the index: %v", index)
			return nil, err
		}
		indexNames = append(indexNames, val.(string))

	}
	log.Debugf("OpenSearch: Found Verrazzano indices %v", indices)
	return indexNames, nil
}

func (o *OSClient) reindexAndDeleteIndices(log vzlog.VerrazzanoLogger, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance,
	openSearchEndpoint string, indices []string, isSystemIndex bool) error {
	for _, index := range indices {
		var dataStreamName string
		if isSystemIndex {
			dataStreamName = systemDataStreamName
		} else {
			dataStreamName = strings.Replace(index, "verrazzano-namespace", "verrazzano-application", 1)
		}
		noOfSecs, err := getRetentionAgeInSeconds(log, vmi, dataStreamName)
		if err != nil {
			return err
		}
		log.Infof("OpenSearch: Reindexing logs from index %v to data stream %s", index, dataStreamName)
		err = o.reindexToDataStream(log, openSearchEndpoint, index, dataStreamName, noOfSecs)
		if err != nil {
			return err
		}
		log.Infof("OpenSearch: Deleting old index %v", index)
		err = o.deleteIndex(log, openSearchEndpoint, index)
		if err != nil {
			return err
		}
		log.Infof("OpenSearch: Deleted old index %v successfully", index)
	}
	return nil
}

func (o *OSClient) reindexToDataStream(log vzlog.VerrazzanoLogger, openSearchEndpoint string,
	sourceName string, destName string, retentionDays string) error {
	var payload string
	var err error
	if retentionDays == "" {
		input := ReindexInputWithoutQuery{SourceName: sourceName, DestinationName: destName}
		payload, err = formatReindexPayloadWithoutQuery(input, reindexPayloadWithoutQuery)
	} else {
		input := ReindexInput{SourceName: sourceName, DestinationName: destName, NumberOfSeconds: retentionDays}
		payload, err = formatReindexPayload(input, reindexPayload)
	}
	if err != nil {
		return err
	}

	reindexURL := fmt.Sprintf("%s/_reindex", openSearchEndpoint)
	log.Debugf("OpenSearch: Executing delete index API %s", reindexURL)
	req, err := http.NewRequest("POST", reindexURL, strings.NewReader(payload))
	if err != nil {
		return err
	}

	resp, err := o.DoHTTP(req)
	if err != nil {
		log.Errorf("OpenSearch: Reindex from %s to %s failed", sourceName, destName)
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d when reindexing from %s to %s failed", resp.StatusCode, sourceName,
			destName)
	}
	responseBody, _ := ioutil.ReadAll(resp.Body)
	log.Debugf("OpenSearch: Reindex from %s to %s API response %s", sourceName, destName, string(responseBody))
	return nil
}

func calculateSeconds(age string) (uint64, error) {
	match := reTimeUnit.FindStringSubmatch(age)
	if match == nil || len(match) < 2 {
		return 0, fmt.Errorf("unable to convert %s to seconds due to invalid format", age)
	}
	n := match[1]
	number, err := strconv.ParseUint(n, 10, 0)
	if err != nil {
		return 0, fmt.Errorf("unable to parse the specified time unit %s", n)
	}
	switch match[2] {
	case "w":
		return number * secondsPerWeek, nil
	case "d":
		return number * secondsPerDay, nil
	case "h", "H":
		return number * secondsPerHour, nil
	case "m":
		return number * secondsPerMinute, nil
	case "s":
		return number, nil
	}
	return 0, fmt.Errorf("conversion to seconds for time unit %s is unsupported", match[2])
}

func (o *OSClient) deleteIndex(log vzlog.VerrazzanoLogger, openSearchEndpoint string, indexName string) error {
	deleteIndexURL := fmt.Sprintf("%s/%s", openSearchEndpoint, indexName)
	log.Debugf("OpenSearch: Executing delete index API %s", deleteIndexURL)
	req, err := http.NewRequest("DELETE", deleteIndexURL, nil)
	if err != nil {
		return err
	}

	resp, err := o.DoHTTP(req)
	if err != nil {
		log.Debugf("OpenSearch: Delete API failed %v", err)
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d when deleting the indice %s in OpenSearch", resp.StatusCode, indexName)
	}
	responseBody, _ := ioutil.ReadAll(resp.Body)
	log.Debugf("OpenSearch: Delete API response %s", string(responseBody))
	return nil
}

func formatReindexPayload(input ReindexInput, payload string) (string, error) {
	tmpl, err := template.New("reindex").
		Option("missingkey=error").
		Parse(payload)
	if err != nil {
		return "", err
	}
	buffer := &bytes.Buffer{}
	if err := tmpl.Execute(buffer, input); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func formatReindexPayloadWithoutQuery(input ReindexInputWithoutQuery, payload string) (string, error) {
	tmpl, err := template.New("reindexWithoutQuery").
		Option("missingkey=error").
		Parse(payload)
	if err != nil {
		return "", err
	}
	buffer := &bytes.Buffer{}
	if err := tmpl.Execute(buffer, input); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func getRetentionAgeInSeconds(log vzlog.VerrazzanoLogger, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance,
	indexName string) (string, error) {
	var noOfSecs string
	for _, policy := range vmi.Spec.Elasticsearch.Policies {
		regexpString := resources.ConvertToRegexp(policy.IndexPattern)
		matched, _ := regexp.MatchString(regexpString, indexName)
		if matched {
			noOfSecs, err := calculateSeconds(*policy.MinIndexAge)
			if err != nil {
				return fmt.Sprintf("%ds", noOfSecs), nil
			} else {
				return "", log.ErrorfNewErr("Failed in OpenSearch post upgrade: error in calculating the number of"+
					" seconds of past application logs that has to be re-indexed: %v", err)
			}
		}
	}
	return noOfSecs, nil
}
