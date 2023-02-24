// Copyright (C) 2022, 2023, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package dashboards

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
)

var defaultIndexPatterns = [...]string{constants.VZSystemIndexPattern, constants.VZAppIndexPattern}

// SavedObjectType specifies the OpenSearch SavedObject including index-patterns.
type SavedObjectType struct {
	Type       string `json:"type"`
	Attributes `json:"attributes"`
}

// CreateDefaultIndexPatterns creates the defaultIndexPatterns in the OpenSearchDashboards if not existed
func (od *OSDashboardsClient) CreateDefaultIndexPatterns(log vzlog.VerrazzanoLogger, openSearchDashboardsEndpoint string) error {
	existingIndexPatterns, err := od.getDefaultIndexPatterns(openSearchDashboardsEndpoint, 50, fmt.Sprintf("%s+or+%s", strings.Replace(constants.VZSystemIndexPattern, "*", "\\*", -1), strings.Replace(constants.VZAppIndexPattern, "*", "\\*", -1)))
	if err != nil {
		return err
	}
	var savedObjectPayloads []SavedObjectType
	for _, indexPattern := range defaultIndexPatterns {
		if existingIndexPatterns[indexPattern] {
			continue
		}
		savedObject := SavedObjectType{
			Type: constants.IndexPattern,
			Attributes: Attributes{
				Title:         indexPattern,
				TimeFieldName: constants.TimeStamp,
			},
		}
		savedObjectPayloads = append(savedObjectPayloads, savedObject)
	}
	if len(savedObjectPayloads) > 0 {
		log.Progressf("Creating default index patterns")
		err = od.creatIndexPatterns(log, savedObjectPayloads, openSearchDashboardsEndpoint)
		if err != nil {
			return err
		}
	}
	return nil
}

// creatIndexPatterns creates the given IndexPattern in the OpenSearch-Dashboards by calling bulk API.
func (od *OSDashboardsClient) creatIndexPatterns(log vzlog.VerrazzanoLogger, savedObjectList []SavedObjectType, openSearchDashboardsEndpoint string) error {
	savedObjectBytes, err := json.Marshal(savedObjectList)
	if err != nil {
		return err
	}
	indexPatternURL := fmt.Sprintf("%s/api/saved_objects/_bulk_create", openSearchDashboardsEndpoint)
	req, err := http.NewRequest("POST", indexPatternURL, strings.NewReader(string(savedObjectBytes)))
	if err != nil {
		log.Errorf("failed to create request for index patterns using bulk API %s", err.Error())
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("osd-xsrf", "true")
	resp, err := od.DoHTTP(req)
	if err != nil {
		log.Errorf("failed to create index patterns %s using bulk API %s", string(savedObjectBytes), err.Error())
		return fmt.Errorf("failed to post index patterns in OpenSearch dashboards: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post status code %d when creating index patterns: %s", resp.StatusCode, string(savedObjectBytes))
	}
	return nil
}

// getDefaultIndexPatterns fetches the existing defaultIndexPatterns.
func (od *OSDashboardsClient) getDefaultIndexPatterns(openSearchDashboardsEndpoint string, perPage int, searchQuery string) (map[string]bool, error) {
	defaultIndexPatternMap := map[string]bool{}
	savedObjects, err := od.getPatterns(openSearchDashboardsEndpoint, perPage, searchQuery)
	if err != nil {
		return defaultIndexPatternMap, err
	}
	for _, savedObject := range savedObjects {
		if isDefaultIndexPattern(savedObject.Title) {
			defaultIndexPatternMap[savedObject.Title] = true
		}
	}
	return defaultIndexPatternMap, nil
}

// isDefaultIndexPattern checks whether given index pattern is default index pattern or not
func isDefaultIndexPattern(indexPattern string) bool {
	for _, defaultIndexPattern := range defaultIndexPatterns {
		if defaultIndexPattern == indexPattern {
			return true
		}
	}
	return false
}
