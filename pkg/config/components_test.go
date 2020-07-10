// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestNoImages(t *testing.T) {
	err := InitComponentDetails()
	assert.Error(t, err)
}

func TestAllImages(t *testing.T) {
	//create log for image tester
	logger := zerolog.New(os.Stderr).With().Timestamp().Str("kind", "TestAllImages").Str("name", "testing").Logger()

	// Create environment variable for each component
	for _, component := range AllComponentDetails {
		if len(component.EnvName) > 0 {
			logger.Info().Msgf("Setting environment variable %s", component.EnvName)
			err := os.Setenv(component.EnvName, "TEST")
			assert.Nil(t, err, fmt.Sprintf("setting environment variable %s", component.EnvName))
		}
	}
	err := os.Setenv(ESWaitTargetVersionEnv, "es.TEST")
	assert.Nil(t, err, fmt.Sprintf("setting environment variable %s", ESWaitTargetVersionEnv))

	err = InitComponentDetails()
	assert.Nil(t, err, "Expected initComponentDetails to succeed")

	// Test the image names were set as expected
	for _, component := range AllComponentDetails {
		if len(component.EnvName) > 0 {
			assert.Equal(t, "TEST", component.Image, fmt.Sprintf("checking image name field for %s", component.Name))
		}
	}
}
