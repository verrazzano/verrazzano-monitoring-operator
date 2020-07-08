// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package logs

import (
	"strconv"
	"os"
	"github.com/rs/zerolog"
)

// Initialize logs with Time and Global Level of Logs set at Info
func InitLogs() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	// Log levels are outlined as follows:
	// Panic: 5
	// Fatal: 4
	// Error: 3
	// Warn: 2
	// Info: 1
	// Debug: 0
	// Trace: -1
	// more info can be found at https://github.com/rs/zerolog#leveled-logging

	envLog := os.Getenv("LOG_LEVEL")
	if val, err := strconv.Atoi(envLog); envLog != "" && err == nil && val >= -1 && val <= 5 {
		zerolog.SetGlobalLevel(zerolog.Level(val))
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
