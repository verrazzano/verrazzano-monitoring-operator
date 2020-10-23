// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package logs

import (
	"github.com/rs/zerolog"
	"os"
	"strconv"
	"time"
)

// InitLogs initializes logs with Time and Global Level of Logs set at Info
func InitLogs() {
	zerolog.TimeFieldFormat = time.RFC3339

	// Log levels can be found at https://github.com/rs/zerolog#leveled-logging
	envLog := os.Getenv("LOG_LEVEL")
	if val, err := strconv.Atoi(envLog); envLog != "" && err == nil && val >= -1 && val <= 5 {
		zerolog.SetGlobalLevel(zerolog.Level(val))
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}
