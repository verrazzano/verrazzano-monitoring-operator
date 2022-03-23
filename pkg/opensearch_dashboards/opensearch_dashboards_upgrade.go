// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package dashboards

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"text/template"
)

const updatePatternPayload = `{
   "attributes": {
     "title": "{{ .IndexPattern }}"
   }
 }
`

type PatternInput struct {
	IndexPattern string
}

var (
	systemNamespaces = []string{"kube-system", "verrazzano-system", "istio-system", "keycloak", "metallb-system",
		"default", "cert-manager", "local-path-storage", "rancher-operator-system", "fleet-system", "ingress-nginx",
		"cattle-system", "verrazzano-install", "monitoring"}
)

const (
	systemDataStreamName = "verrazzano-system"
)

func (od *OSDashboardsClient) updatePatternsInternal(log vzlog.VerrazzanoLogger, dashboardsEndPoint string) error {
	dashboardPatterns, err := od.getPatterns(log, dashboardsEndPoint)
	if err != nil {
		return err
	}
	for id, originalPattern := range dashboardPatterns {
		updatedPattern := constructUpdatedPattern(originalPattern)
		if id == "" || (originalPattern == updatedPattern) {
			continue
		}
		// Invoke update index pattern API
		err = od.executeUpdate(log, dashboardsEndPoint, id, originalPattern, updatedPattern)
		if err != nil {
			log.Infof("OpenSearch Dashboards: Updating index pattern failed: %v", err)
			return err
		}
	}
	return nil
}

func (od *OSDashboardsClient) getPatterns(log vzlog.VerrazzanoLogger, dashboardsEndPoint string) (map[string]string, error) {
	getPatternsURL := fmt.Sprintf("%s/api/saved_objects/_find?type=index-pattern&fields=title", dashboardsEndPoint)
	log.Debugf("OpenSearch Dashboards: Executing Get index patterns API %s", getPatternsURL)
	req, err := http.NewRequest("GET", getPatternsURL, nil)
	if err != nil {
		return nil, err
	}
	getResponse, err := od.DoHTTP(req)
	if err != nil {
		log.Errorf("OpenSearch Dashboards: Get index patterns failed")
		return nil, err
	}
	if getResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got status code %d when getting index patterns", getResponse.StatusCode)
	}

	responseBody, _ := ioutil.ReadAll(getResponse.Body)
	if string(responseBody) == "" {
		log.Debugf("OpenSearch Dashboards: Empty response for get index patterns")
		return nil, nil
	}
	log.Debugf("OpenSearch Dashboards: Get index patterns response %v", string(responseBody))
	var responseMap map[string]interface{}
	if err := json.Unmarshal(responseBody, &responseMap); err != nil {
		log.Errorf("OpenSearch Dashboards: Error unmarshalling index patterns response body: %v", err)
	}
	patterns := make(map[string]string)
	if responseMap["saved_objects"] != nil {
		savedObjects := reflect.ValueOf(responseMap["saved_objects"])
		for i := 0; i < savedObjects.Len(); i++ {
			log.Debugf("OpenSearch Dashboards: Index pattern details: %v", savedObjects.Index(i))
			savedObject := savedObjects.Index(i).Interface().(map[string]interface{})
			attributes := savedObject["attributes"].(map[string]interface{})
			title := attributes["title"].(string)
			id := savedObject["id"]
			patterns[id.(string)] = title
		}
	}
	log.Debugf("OpenSearch Dashboards: Found index patterns in OpenSearch Dashboards %v", patterns)
	return patterns, nil
}

func (od *OSDashboardsClient) executeUpdate(log vzlog.VerrazzanoLogger, dashboardsEndPoint string,
	id string, originalPattern string, updatedPattern string) error {
	input := PatternInput{IndexPattern: updatedPattern}
	payload, err := formatPatternPayload(input, updatePatternPayload)
	if err != nil {
		return err
	}
	log.Infof("OpenSearch Dashboards: Replacing index pattern %s with %s in OpenSearch Dashboards", originalPattern, updatedPattern)
	updatedPatternURL := fmt.Sprintf("%s/api/saved_objects/index-pattern/%s", dashboardsEndPoint, id)
	log.Debugf("OpenSearch Dashboards: Executing update saved object API %s", updatedPatternURL)
	req, err := http.NewRequest("PUT", updatedPatternURL, strings.NewReader(payload))
	if err != nil {
		return err
	}
	resp, err := od.DoHTTP(req)
	if err != nil {
		log.Errorf("OpenSearch Dashboards: Get index patterns failed")
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("got status code %d when getting index patterns", resp.StatusCode)
	}

	responseBody, _ := ioutil.ReadAll(resp.Body)
	log.Debugf("OpenSearch Dashboards: Update index pattern API response %s", responseBody)
	return nil
}

func formatPatternPayload(input PatternInput, payload string) (string, error) {
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

func constructUpdatedPattern(originalPattern string) string {
	var updatedPattern []string
	patternList := strings.Split(originalPattern, ",")
	for _, eachPattern := range patternList {
		if strings.HasPrefix(eachPattern, "verrazzano-") && eachPattern != "verrazzano-*" {
			// To match the exact pattern, add ^ in the beginning and $ in the end
			regexpString := resources.ConvertToRegexp(eachPattern)
			systemIndexMatch := isSystemIndexMatch(regexpString)
			if systemIndexMatch {
				updatedPattern = append(updatedPattern, systemDataStreamName)
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
	for _, namespace := range systemNamespaces {
		systemIndex, _ := regexp.MatchString(pattern, "verrazzano-namespace-"+namespace)
		if systemIndex {
			return true
		}
	}
	return false
}
