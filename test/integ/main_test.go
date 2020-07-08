// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package integ

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/verrazzano/verrazzano-monitoring-operator/test/integ/framework"
	testutil "github.com/verrazzano/verrazzano-monitoring-operator/test/integ/util"
)

var (
	nsCreated = false
)

func TestMain(m *testing.M) {
	//create log for Main test
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "IntegTest").Str("name", "MainTest").Logger()

	// Global setup
	if err := framework.Setup(); err != nil {
		logger.Error().Msgf("Failed to setup framework: %v", err)
		os.Exit(1)
	}
	// Create the namespace if it does not exist as part of global setup (and delete it if we created it in teardown)
	if !testutil.NamespaceExists(framework.Global.Namespace, framework.Global.KubeClient) {
		if err := testutil.CreateNamespace(framework.Global.Namespace, framework.Global.KubeClient); err != nil {
			logger.Error().Msgf("Failed to create namespace %s for test: %v", framework.Global.Namespace, err)
			os.Exit(1)
		} else {
			nsCreated = true
		}
	}

	code := m.Run()

	if nsCreated && !framework.Global.SkipTeardown {
		if err := testutil.DeleteNamespace(framework.Global.Namespace, framework.Global.KubeClient); err != nil {
			logger.Error().Msgf("Failed to clean up integ test namespace: %v", err)
		}
	}
	// Global tear-down
	if err := framework.Teardown(); err != nil {
		logger.Error().Msgf("Failed to teardown framework: %v", err)
		os.Exit(1)
	}
	os.Exit(code)
}
