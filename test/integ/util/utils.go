// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package util

import (
	"fmt"
	"github.com/go-resty/resty/v2"
	"github.com/verrazzano/verrazzano-monitoring-operator/test/integ/framework"
	"os"
)

// RunBeforePhase returns, based on the given framework object, whether or not to run the before phase.
func RunBeforePhase(f *framework.Framework) bool {
	return framework.Before == f.Phase || f.Phase == ""
}

// RunAfterPhase returns, based on the given framework object, whether or not to run the after phase.
func RunAfterPhase(f *framework.Framework) bool {
	return framework.After == f.Phase || f.Phase == ""
}

// SkipTeardown returns, based on the given framework object, whether or not to skip teardown
func SkipTeardown(f *framework.Framework) bool {
	return f.SkipTeardown || framework.Before == f.Phase
}

// GetClient gets a client, setting the proxy if appropriate
func GetClient() *resty.Client {
	restyClient := resty.New()

	f := framework.Global
	// Set proxy for resty client
	if f.ExternalIP != "localhost" {
		proxyURL := os.Getenv("http_proxy")
		if proxyURL != "" {
			fmt.Println("Setting proxy for resty clients to :" + proxyURL)
			restyClient.SetProxy(proxyURL)
		}
	}
	return restyClient
}
