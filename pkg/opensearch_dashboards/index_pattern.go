// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package dashboards

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const VZSystemIndexPattern = "verrazzano-system*"
const VZAppIndexPattern = "verrazzano-application*"

var DefaultIndexPatterns = []string{VZSystemIndexPattern, VZAppIndexPattern}

// CreateIndexPattern creates the DefaultIndexPatterns in the OpenSearchDashboards if not existed
func (od *OSDashboardsClient) CreateIndexPattern(openSearchDashboardsEndpoint string) error {
	defaultIndexPatterns, err := od.getDefaultIndexPatterns(openSearchDashboardsEndpoint, 50, fmt.Sprintf("(%s%20or%20%s)*", VZAppIndexPattern, VZAppIndexPattern))
	if err != nil {
		return err
	}
	var savedObjectPayload []SavedObject
	for _, indexPattern := range DefaultIndexPatterns {
		isPatternExist, ok := defaultIndexPatterns[indexPattern]
		if isPatternExist && ok {
			continue
		}
		savedObject := SavedObject{
			Attributes: Attributes{
				Title: indexPattern,
			},
		}
		savedObjectPayload = append(savedObjectPayload, savedObject)
	}
	savedObjectBytes, err := json.Marshal(savedObjectPayload)
	if err != nil {
		return err
	}
	indexPatternURL := fmt.Sprintf("%s/api/saved_objects/_bulk_create", openSearchDashboardsEndpoint)
	req, err := http.NewRequest("POST", indexPatternURL, strings.NewReader(string(savedObjectBytes)))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("osd-xsrf", "true")
	resp, err := od.DoHTTP(req)
	if err != nil {
		return fmt.Errorf("failed to post index patterns in OpenSearch dashboards: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post status code %d when creating index patterns", resp.StatusCode)
	}
	return nil
}

// getDefaultIndexPatterns fetches the existing DefaultIndexPatterns.
func (od *OSDashboardsClient) getDefaultIndexPatterns(openSearchDashboardsEndpoint string, perPage int, searchQuery string) (map[string]bool, error) {
	defaultIndexPattern := map[string]bool{}
	savedObjects, err := od.getPatterns(openSearchDashboardsEndpoint, perPage, searchQuery)
	if err != nil {
		return defaultIndexPattern, err
	}
	for _, savedObject := range savedObjects {
		if savedObject.Title == VZAppIndexPattern {
			defaultIndexPattern[VZAppIndexPattern] = true
		}
		if savedObject.Title == VZSystemIndexPattern {
			defaultIndexPattern[VZSystemIndexPattern] = true
		}
	}
	return defaultIndexPattern, nil
}
