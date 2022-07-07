// Copyright (c) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package log

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
)

// TestLogger tests the Logger method to create a zap logger
// GIVEN input file name
// WHEN file has been pre-created
// THEN creates zap logger object to be consumed by other methods
func TestLogger(t *testing.T) {
	file, _ := os.CreateTemp(os.TempDir(), fmt.Sprintf("verrazzano-%s-hook-*.log", strings.ToLower("BACKUP")))
	defer file.Close()
	defer os.Remove(file.Name())
	logger, err := Logger(file.Name())
	assert.Nil(t, err)
	assert.NotNil(t, logger)

}
