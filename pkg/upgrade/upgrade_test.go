// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package upgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/opensearch"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/util/logs/vzlog"
	kubeinformers "k8s.io/client-go/informers"
	fake "k8s.io/client-go/kubernetes/fake"
)

// TestMigrateOldIndicesNoErrorWhenOSNotReady tests that MigrateOldIndices does not return error when OpenSearch is not ready
// GIVEN a default VMI instance
// WHEN I call MigrateOldIndices
// THEN the MigrateOldIndices does not return error when the Opensearch StatefulSet pods are not ready
func TestMigrateOldIndicesNoErrorWhenOSNotReady(t *testing.T) {
	// fake statefulSetLister that returns no StatefulSets and hence o.IsOpenSearchReady() returns false
	statefulSetLister := kubeinformers.NewSharedInformerFactory(fake.NewSimpleClientset(), constants.ResyncPeriod).Apps().V1().StatefulSets().Lister()
	o := opensearch.NewOSClient(statefulSetLister)
	monitor := &Monitor{}
	err := monitor.MigrateOldIndices(vzlog.DefaultLogger(), &vmcontrollerv1.VerrazzanoMonitoringInstance{}, o, nil)
	assert.NoError(t, err)
}
