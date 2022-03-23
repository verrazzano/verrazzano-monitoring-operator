// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package upgrade

import (
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/opensearch"
	dashboards "github.com/verrazzano/verrazzano-monitoring-operator/pkg/opensearch_dashboards"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
)

// MigrateOldIndices migrates old Verrazzano indices to data streams
// The returned channel should be read for exactly one response, which tells whether migration succeeded.
func MigrateOldIndices(log vzlog.VerrazzanoLogger, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance,
	o *opensearch.OSClient, od *dashboards.OSDashboardsClient) chan error {
	ch := make(chan error)
	// configuration is done asynchronously, as this does not need to be blocking
	go func() {
		if !vmi.Spec.Elasticsearch.Enabled {
			ch <- nil
			return
		}
		openSearchEndpoint := resources.GetOpenSearchHTTPEndpoint(vmi)
		// During upgrade, reindex and delete old indices
		if err := o.MigrateIndicesToDataStreams(log, vmi, openSearchEndpoint); err != nil {
			ch <- err
			return
		}

		// Update if any index patterns configured for old indices in OpenSearch Dashboards
		err := od.UpdatePatterns(log, vmi)
		if err != nil {
			ch <- log.ErrorfNewErr("Error in updating index patterns"+
				" in OpenSearch Dashboards: %v", err)
			return
		}

		ch <- nil
	}()

	return ch
}
