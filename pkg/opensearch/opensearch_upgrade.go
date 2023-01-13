// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package opensearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
)

var (
	secondsPerMinute = uint64(60)
	secondsPerHour   = secondsPerMinute * 60
	secondsPerDay    = secondsPerHour * 24
	unitMultipliers  = map[uint8]uint64{
		's': 1,
		'm': secondsPerMinute,
		'h': secondsPerHour,
		'd': secondsPerDay,
	}
)

type (
	ReindexPayload struct {
		Conflicts string `json:"conflicts"`
		Source    `json:"source"`
		Dest      `json:"dest"`
	}

	Source struct {
		Index string `json:"index"`
		Query *Query `json:"query,omitempty"`
	}

	Query struct {
		Range `json:"range"`
	}

	Range struct {
		Timestamp `json:"@timestamp"`
	}

	Timestamp struct {
		GreaterThanEqual string `json:"gte"`
		LessThan         string `json:"lt"`
	}

	Dest struct {
		Index  string `json:"index"`
		OpType string `json:"op_type"`
	}
)

// Reindex old style indices to data streams and delete it
func (o *OSClient) MigrateIndicesToDataStreams(log vzlog.VerrazzanoLogger, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, openSearchEndpoint string) error {
	log.Debugf("Checking for OpenSearch indices to migrate to data streams.")
	// Get the indices
	indices, err := o.getIndices(log, openSearchEndpoint)
	if err != nil {
		return fmt.Errorf("failed to retrieve OpenSearch index list: %v", err)
	}
	systemIndices := getSystemIndices(log, indices)
	appIndices := getApplicationIndices(log, indices)

	if len(systemIndices) > 0 || len(appIndices) > 0 {
		log.Info("Migrating Verrazzano indices to data streams")
	}

	// Reindex and delete old system indices
	err = o.reindexAndDeleteIndices(log, vmi, openSearchEndpoint, systemIndices, true)
	if err != nil {
		return fmt.Errorf("failed to migrate the Verrazzano system indices to data streams: %v", err)
	}

	// Reindex and delete old application indices
	err = o.reindexAndDeleteIndices(log, vmi, openSearchEndpoint, appIndices, false)
	if err != nil {
		return fmt.Errorf("failed to migrate the Verrazzano application indices to data streams: %v", err)
	}
	if len(systemIndices) > 0 || len(appIndices) > 0 {
		log.Info("Migration of Verrazzano indices to data streams completed successfully")
	} else {
		log.Debug("Found no indices to migrate to data streams")
	}
	return nil
}

func (o *OSClient) DataStreamExists(openSearchEndpoint, dataStream string) (bool, error) {
	url := fmt.Sprintf("%s/_data_stream/%s", openSearchEndpoint, dataStream)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, err
	}
	resp, err := o.DoHTTP(req)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("got code %d when checking for data stream %s", resp.StatusCode, config.DataStreamName())
	}
	return true, nil
}

func getSystemIndices(log vzlog.VerrazzanoLogger, indices []string) []string {
	var systemIndices []string
	for _, index := range indices {
		if strings.HasPrefix(index, "verrazzano-namespace-") {
			for _, systemNamespace := range config.SystemNamespaces() {
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
	log.Debugf("Found Verrazzano system indices %v", systemIndices)
	return systemIndices
}

func getApplicationIndices(log vzlog.VerrazzanoLogger, indices []string) []string {
	var appIndices []string
	for _, index := range indices {
		systemIndex := false
		if strings.HasPrefix(index, "verrazzano-namespace-") {
			for _, systemNamespace := range config.SystemNamespaces() {
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
	log.Debugf("Found Verrazzano application indices %v", appIndices)
	return appIndices
}

func (o *OSClient) getIndices(log vzlog.VerrazzanoLogger, openSearchEndpoint string) ([]string, error) {
	indicesURL := fmt.Sprintf("%s/_aliases", openSearchEndpoint)
	log.Debugf("Executing get indices API %s", indicesURL)
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
	var indices map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&indices)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall indices response: %v", err)
	}
	var indexNames []string
	for index := range indices {
		indexNames = append(indexNames, index)

	}
	log.Debugf("Found Verrazzano indices %v", indices)
	return indexNames, nil
}

func (o *OSClient) reindexAndDeleteIndices(log vzlog.VerrazzanoLogger, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance,
	openSearchEndpoint string, indices []string, isSystemIndex bool) error {
	for _, index := range indices {
		var dataStreamName string
		if isSystemIndex {
			dataStreamName = config.DataStreamName()
		} else {
			dataStreamName = strings.Replace(index, "verrazzano-namespace", "verrazzano-application", 1)
		}
		noOfSecs, err := getRetentionAgeInSeconds(vmi, dataStreamName)
		if err != nil {
			return err
		}
		log.Infof("Reindexing data from index %v to data stream %s", index, dataStreamName)
		err = o.reindexToDataStream(log, openSearchEndpoint, index, dataStreamName, noOfSecs)
		if err != nil {
			return err
		}
		log.Infof("Cleaning up index %v", index)
		err = o.deleteIndex(log, openSearchEndpoint, index)
		if err != nil {
			return err
		}
		log.Infof("Successfully cleaned up index %v", index)
	}
	return nil
}

func (o *OSClient) reindexToDataStream(log vzlog.VerrazzanoLogger, openSearchEndpoint, sourceName, destName, retentionSeconds string) error {
	reindexPayload := createReindexPayload(sourceName, destName, retentionSeconds)
	payload, err := json.Marshal(reindexPayload)
	if err != nil {
		return err
	}
	reindexURL := fmt.Sprintf("%s/_reindex", openSearchEndpoint)
	log.Debugf("Executing Reindex API %s", reindexURL)

	req, err := http.NewRequest("POST", reindexURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Add(contentTypeHeader, applicationJSON)
	resp, err := o.DoHTTP(req)
	if err != nil {
		log.Errorf("Reindex from %s to %s failed", sourceName, destName)
		return err
	}
	defer resp.Body.Close()
	responseBody, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d when reindexing from %s to %s failed: %s", resp.StatusCode, sourceName,
			destName, string(responseBody))
	}

	log.Infof("Reindex from %s to %s completed successfully: %s", sourceName, destName, string(responseBody))
	return nil
}

func (o *OSClient) deleteIndex(log vzlog.VerrazzanoLogger, openSearchEndpoint string, indexName string) error {
	deleteIndexURL := fmt.Sprintf("%s/%s", openSearchEndpoint, indexName)
	log.Debugf("Executing delete index API %s", deleteIndexURL)
	req, err := http.NewRequest("DELETE", deleteIndexURL, nil)
	if err != nil {
		return err
	}

	resp, err := o.DoHTTP(req)
	if err != nil {
		return fmt.Errorf("failed to delete index %s: %v", indexName, err)
	}
	defer resp.Body.Close()
	responseBody, _ := ioutil.ReadAll(resp.Body)
	log.Debugf("Delete API response %s", string(responseBody))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d when deleting the indice %s in OpenSearch: %s", resp.StatusCode, indexName, string(responseBody))
	}
	return nil
}

func createReindexPayload(source, dest, retentionSeconds string) *ReindexPayload {
	reindexPayload := &ReindexPayload{
		Conflicts: "proceed",
		Source: Source{
			Index: source,
		},
		Dest: Dest{
			Index:  dest,
			OpType: "create",
		},
	}
	if retentionSeconds != "" {
		reindexPayload.Source.Query = &Query{
			Range: Range{
				Timestamp: Timestamp{
					GreaterThanEqual: fmt.Sprintf("now-%s", retentionSeconds),
					LessThan:         "now/s",
				},
			},
		}
	}

	return reindexPayload
}

func getRetentionAgeInSeconds(vmi *vmcontrollerv1.VerrazzanoMonitoringInstance, indexName string) (string, error) {
	for _, policy := range vmi.Spec.Elasticsearch.Policies {
		regexpString := resources.ConvertToRegexp(policy.IndexPattern)
		matched, _ := regexp.MatchString(regexpString, indexName)
		if matched {
			seconds, err := calculateSeconds(*policy.MinIndexAge)
			if err != nil {
				return "", fmt.Errorf("failed to calculate the retention age in seconds: %v", err)
			}
			return fmt.Sprintf("%ds", seconds), nil

		}
	}
	return "", nil
}

func calculateSeconds(age string) (uint64, error) {
	n := age[:len(age)-1]
	number, err := strconv.ParseUint(n, 10, 0)
	if err != nil {
		return 0, fmt.Errorf("unable to parse the specified time unit %s", n)
	}
	unit := age[len(age)-1]
	result := number * unitMultipliers[unit]
	if result < 1 {
		return result, fmt.Errorf("conversion to seconds for time unit %s is unsupported", strconv.Itoa(int(unit)))
	}
	return result, nil
}
