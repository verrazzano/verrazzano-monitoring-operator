// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package upgrade

import (
	"fmt"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/opensearch"
	dashboards "github.com/verrazzano/verrazzano-monitoring-operator/pkg/opensearch_dashboards"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
)

type Monitor struct {
	running bool
	ch      chan error
}

func (m *Monitor) MigrateOldIndices(log vzlog.VerrazzanoLogger, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance,
	o *opensearch.OSClient, od *dashboards.OSDashboardsClient) error {
	if !vmi.Spec.Opensearch.Enabled || !o.IsOpenSearchReady(vmi) {
		return nil
	}

	// if not already migrating, start migrating indices
	if !m.running {
		m.run(log, vmi, o, od)
		return nil
	}

	complete, err := m.isUpgradeComplete()
	// an error occurred during reindex
	if err != nil {
		// reset the monitor so we can retry the upgrade
		m.reset()
		return err
	}
	// reindex is still in progress
	if !complete {
		log.Info("Data stream reindex is in progress.")
		return nil
	}
	// reindex was successful
	m.reset()
	return nil
}

func (m *Monitor) isUpgradeComplete() (bool, error) {
	if m.ch == nil {
		return false, nil
	}

	select {
	case e := <-m.ch:
		return true, e
	default:
		return false, nil
	}
}

func (m *Monitor) reset() {
	m.running = false
	close(m.ch)
}

func (m *Monitor) run(log vzlog.VerrazzanoLogger, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance,
	o *opensearch.OSClient, od *dashboards.OSDashboardsClient) {
	ch := make(chan error)
	m.running = true
	m.ch = ch
	// configuration is done asynchronously, as this does not need to be blocking
	go func() {
		if !vmi.Spec.Opensearch.Enabled {
			ch <- nil
			return
		}

		openSearchEndpoint := resources.GetOpenSearchHTTPEndpoint(vmi)
		// Make sure that the data stream template is created before re-indexing
		exists, err := o.DataStreamExists(openSearchEndpoint, config.DataStreamName())
		if err != nil {
			ch <- fmt.Errorf("failed to verify existence of data stream: %v", err)
			return
		}

		// If the migration data stream exists, the old backing indices must be reindexed
		if exists {
			// During upgrade, reindex and delete old indices
			if err := o.MigrateIndicesToDataStreams(log, vmi, openSearchEndpoint); err != nil {
				ch <- err
				return
			}

			// Update if any index patterns configured for old indices in OpenSearch Dashboards
			err = od.UpdatePatterns(log, vmi)
			if err != nil {
				ch <- fmt.Errorf("error in updating index patterns"+
					" in OpenSearch Dashboards: %v", err)
				return
			}
		}
		ch <- nil
	}()
}
