// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package dashboards

import (
	"fmt"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	"net/http"
)

type (
	OSDashboardsClient struct {
		httpClient *http.Client
		DoHTTP     func(request *http.Request) (*http.Response, error)
	}
)

func NewOSDashboardsClient() *OSDashboardsClient {
	od := &OSDashboardsClient{
		httpClient: http.DefaultClient,
	}
	od.DoHTTP = func(request *http.Request) (*http.Response, error) {
		return od.httpClient.Do(request)
	}
	return od
}

// UpdatePatterns updates the index patterns configured for old indices if any to match the corresponding data streams.
func (od *OSDashboardsClient) UpdatePatterns(log vzlog.VerrazzanoLogger, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) error {
	if !vmi.Spec.Kibana.Enabled {
		log.Debugf("OpenSearch Dashboards is not configured to run. Skipping the post upgrade step for OpenSearch Dashboards")
		return nil
	}
	openSearchDashboardsEndpoint := resources.GetOpenSearchDashboardsHTTPEndpoint(vmi)
	// Update index patterns in OpenSearch dashboards
	err := od.updatePatternsInternal(log, openSearchDashboardsEndpoint)
	if err != nil {
		return fmt.Errorf("failed to update index patterns: %v", err)
	}
	return nil
}
