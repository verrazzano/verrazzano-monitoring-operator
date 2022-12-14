// Copyright (C) 2022, Oracle and/or its affiliates.
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
	var savedObjectPayload []SavedObjectType
	for _, indexPattern := range defaultIndexPatterns {
		isPatternExist, ok := existingIndexPatterns[indexPattern]
		if isPatternExist && ok {
			continue
		}
		savedObject := SavedObjectType{
			Type: constants.IndexPattern,
			Attributes: Attributes{
				Title: indexPattern,
			},
		}
		savedObjectPayload = append(savedObjectPayload, savedObject)
	}
	if len(savedObjectPayload) > 0 {
		err = od.creatIndexPatterns(log, savedObjectPayload, openSearchDashboardsEndpoint)
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
	log.Infof("Creating default index patterns")
	indexPatternURL := fmt.Sprintf("%s/api/saved_objects/_bulk_create", openSearchDashboardsEndpoint)
	req, err := http.NewRequest("POST", indexPatternURL, strings.NewReader(string(savedObjectBytes)))
	if err != nil {
		log.Errorf("failed to create bulk request for default index patterns %s", err.Error())
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("osd-xsrf", "true")
	resp, err := od.DoHTTP(req)
	if err != nil {
		log.Errorf("failed to create bulk index patterns %s", err.Error())
		return fmt.Errorf("failed to post index patterns in OpenSearch dashboards: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post status code %d when creating index patterns", resp.StatusCode)
	}
	return nil
}

// getDefaultIndexPatterns fetches the existing defaultIndexPatterns.
func (od *OSDashboardsClient) getDefaultIndexPatterns(openSearchDashboardsEndpoint string, perPage int, searchQuery string) (map[string]bool, error) {
	defaultIndexPattern := map[string]bool{}
	savedObjects, err := od.getPatterns(openSearchDashboardsEndpoint, perPage, searchQuery)
	if err != nil {
		return defaultIndexPattern, err
	}
	for _, savedObject := range savedObjects {
		if savedObject.Title == constants.VZSystemIndexPattern {
			defaultIndexPattern[constants.VZSystemIndexPattern] = true
		}
		if savedObject.Title == constants.VZAppIndexPattern {
			defaultIndexPattern[constants.VZAppIndexPattern] = true
		}
	}
	return defaultIndexPattern, nil
}
