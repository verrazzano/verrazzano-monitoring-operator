// Copyright (C) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package dashboards

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
)

const updatePatternPayload = `{"attributes":{"title":"%s"}}`

type (
	IndexPatterns struct {
		Total        int           `json:"total"`
		Page         int           `json:"page"`
		SavedObjects []SavedObject `json:"saved_objects,omitempty"`
	}

	SavedObject struct {
		ID         string `json:"id"`
		Attributes `json:"attributes"`
	}

	Attributes struct {
		Title         string `json:"title"`
		TimeFieldName string `json:"timeFieldName,omitempty"`
	}
)

func (od *OSDashboardsClient) updatePatternsInternal(log vzlog.VerrazzanoLogger, dashboardsEndPoint string) error {
	// Get index patterns configured in OpenSearch Dashboards
	savedObjects, err := od.getPatterns(dashboardsEndPoint, 100, "")
	if err != nil {
		return err
	}
	existingSavedObjectMap := getSavedObjectMap(savedObjects)
	for i, savedObject := range savedObjects {
		updatedPattern := constructUpdatedPattern(savedObject.Title)
		if updatedPattern == "" || savedObject.Title == updatedPattern {
			continue
		}
		if nil != existingSavedObjectMap[updatedPattern] {
			log.Info("deleting index pattern ", savedObject.Title)
			err = od.deleteIndexPattern(log, dashboardsEndPoint, savedObject.ID, savedObject.Title)
			if err != nil {
				return fmt.Errorf("failed to delete index pattern %s: %v", savedObject.Title, err)
			}
			continue
		}
		// Invoke update index pattern API
		err = od.executeUpdate(log, dashboardsEndPoint, savedObject.ID, savedObject.Title, updatedPattern)
		if err != nil {
			return fmt.Errorf("failed to updated index pattern %s: %v", savedObject.Title, err)
		}
		savedObject.Title = updatedPattern
		existingSavedObjectMap[updatedPattern] = &savedObjects[i]
	}
	return nil
}

// getSavedObjectMap converts list of SavedObject into Map having SavedObject.Title as key and SavedObject as value.
func getSavedObjectMap(savedObjects []SavedObject) map[string]*SavedObject {
	savedObjectMap := map[string]*SavedObject{}
	for i, savedObject := range savedObjects {
		savedObjectMap[savedObject.Title] = &savedObjects[i]
	}
	return savedObjectMap
}

func (od *OSDashboardsClient) getPatterns(dashboardsEndPoint string, perPage int, searchQuery string) ([]SavedObject, error) {
	var savedObjects []SavedObject
	currentPage := 1

	// Index Pattern is a paginated response type, so we need to page out all data
	for {
		url := fmt.Sprintf("%s/api/saved_objects/_find?type=index-pattern&fields=title&per_page=%d&page=%d", dashboardsEndPoint, perPage, currentPage)
		if searchQuery != "" {
			url = fmt.Sprintf("%s/api/saved_objects/_find?search=%s&type=index-pattern&fields=title&per_page=%d&page=%d", dashboardsEndPoint, searchQuery, perPage, currentPage)
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := od.DoHTTP(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("got code %d when querying index patterns", resp.StatusCode)
		}
		indexPatterns := &IndexPatterns{}
		if err := json.NewDecoder(resp.Body).Decode(indexPatterns); err != nil {
			return nil, fmt.Errorf("failed to decode index pattern response body: %v", err)
		}
		currentPage++
		savedObjects = append(savedObjects, indexPatterns.SavedObjects...)
		// paginate responses until we have all the index patterns
		if len(savedObjects) >= indexPatterns.Total {
			break
		}
	}

	return savedObjects, nil
}

func (od *OSDashboardsClient) executeUpdate(log vzlog.VerrazzanoLogger, dashboardsEndPoint string,
	id string, originalPattern string, updatedPattern string) error {
	payload := createIndexPatternPayload(updatedPattern)
	log.Infof("Replacing index pattern %s with %s in OpenSearch Dashboards", originalPattern, updatedPattern)
	updatedPatternURL := fmt.Sprintf("%s/api/saved_objects/index-pattern/%s", dashboardsEndPoint, id)
	log.Debugf("Executing update saved object API %s", updatedPatternURL)
	req, err := http.NewRequest("PUT", updatedPatternURL, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("osd-xsrf", "true")
	resp, err := od.DoHTTP(req)
	if err != nil {
		return fmt.Errorf("failed to get index patterns from OpenSearch dashboards: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d when getting index patterns", resp.StatusCode)
	}

	responseBody, _ := ioutil.ReadAll(resp.Body)
	log.Debugf("Response from OpenSearch Dashboards update index API: %s", responseBody)
	return nil
}

// deleteIndexPattern deletes the index pattern with given index pattern ID.
func (od *OSDashboardsClient) deleteIndexPattern(log vzlog.VerrazzanoLogger, dashboardsEndPoint string,
	id string, indexPattern string) error {
	log.Infof("Deleting index pattern %s in OpenSearch Dashboards", indexPattern)
	updatedPatternURL := fmt.Sprintf("%s/api/saved_objects/index-pattern/%s", dashboardsEndPoint, id)
	log.Debugf("Executing delete saved object API %s", updatedPatternURL)
	req, err := http.NewRequest("DELETE", updatedPatternURL, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("osd-xsrf", "true")
	resp, err := od.DoHTTP(req)
	if err != nil {
		return fmt.Errorf("failed to delete index patterns from OpenSearch dashboards: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d when deleting index patterns", resp.StatusCode)
	}
	return nil
}

func createIndexPatternPayload(indexPattern string) string {
	return fmt.Sprintf(updatePatternPayload, indexPattern)
}

/*
 * constructUpdatedPattern constructs the updated pattern as follows:
 * - Update index patterns matching old system indices to match data stream verrazzano-system
 * - Update index patterns matching old application indices verrazzano-namespace-<application namespace>
 *   to match data stream verrazzano-application-<application namespace>
 */
func constructUpdatedPattern(originalPattern string) string {
	var updatedPattern []string
	patternList := strings.Split(originalPattern, ",")
	for _, eachPattern := range patternList {
		if strings.HasPrefix(eachPattern, "verrazzano-") && eachPattern != "verrazzano-*" {
			// To match the exact pattern, add ^ in the beginning and $ in the end
			regexpString := resources.ConvertToRegexp(eachPattern)
			systemIndexMatch := isSystemIndexMatch(regexpString)
			if systemIndexMatch {
				updatedPattern = append(updatedPattern, config.DataStreamName())
			}
			isNamespaceIndexMatch, _ := regexp.MatchString(regexpString, "verrazzano-namespace-")
			if isNamespaceIndexMatch {
				updatedPattern = append(updatedPattern, "verrazzano-application-*")
			} else if strings.HasPrefix(eachPattern, "verrazzano-namespace-") {
				// If the pattern matches system index and no * present in the pattern, then it is considered as only
				// system index
				if systemIndexMatch && !strings.Contains(eachPattern, "*") {
					continue
				}
				updatedPattern = append(updatedPattern, strings.Replace(eachPattern, "verrazzano-namespace-", "verrazzano-application-", 1))
			}
		} else {
			updatedPattern = append(updatedPattern, eachPattern)
		}
	}
	return strings.Join(updatedPattern, ",")
}

func isSystemIndexMatch(pattern string) bool {
	logStashIndex, _ := regexp.MatchString(pattern, "verrazzano-logstash-")
	systemJournalIndex, _ := regexp.MatchString(pattern, "verrazzano-systemd-journal")
	if logStashIndex || systemJournalIndex {
		return true
	}
	for _, namespace := range config.SystemNamespaces() {
		systemIndex, _ := regexp.MatchString(pattern, "verrazzano-namespace-"+namespace)
		if systemIndex {
			return true
		}
	}
	return false
}
