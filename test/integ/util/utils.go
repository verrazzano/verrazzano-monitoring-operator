// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package util

import (
	"github.com/verrazzano/verrazzano-monitoring-operator/test/integ/framework"
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
