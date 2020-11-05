// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package logs

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	kzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// InitLogs initializes logs with Time and Global Level of Logs set at Info
func InitLogs(opts kzap.Options) {
	var config zap.Config
	if opts.Development {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
	}
	if opts.Level != nil {
		config.Level = opts.Level.(zap.AtomicLevel)
	} else {
		config.Level.SetLevel(zapcore.InfoLevel)
	}
	config.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	config.EncoderConfig.TimeKey = "@timestamp"
	config.EncoderConfig.MessageKey = "message"
	logger, err := config.Build()
	if err != nil {
		zap.S().Errorf("Error creating logger %v", err)
	} else {
		zap.ReplaceGlobals(logger)
	}
}
