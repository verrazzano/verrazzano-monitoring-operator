// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package config

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNoImages(t *testing.T) {
	unsetEnvVars(t, AllComponentDetails)
	err := InitComponentDetails()
	assert.Error(t, err)
}

func TestAllImages(t *testing.T) {
	createEnvVars(t, AllComponentDetails)
	testImages(t, AllComponentDetails, nil)
}

func TestOptionalImages(t *testing.T) {
	var components = []*ComponentDetails{}
	var optionalComponents = []*ComponentDetails{}

	// separate the optional components
	for _, component := range AllComponentDetails {
		if component.Optional {
			optionalComponents = append(optionalComponents, component)
		} else {
			components = append(components, component)
		}
	}
	createEnvVars(t, components)
	unsetEnvVars(t, optionalComponents)
	testImages(t, components, optionalComponents)
}

func createEnvVars(t *testing.T, components []*ComponentDetails) {
	// Create environment variable for each component
	for _, component := range components {
		if len(component.EnvName) > 0 {
			zap.S().Infof("Setting environment variable %s", component.EnvName)
			err := os.Setenv(component.EnvName, "TEST")
			assert.Nil(t, err, fmt.Sprintf("setting environment variable %s", component.EnvName))
		}
	}
}

func unsetEnvVars(t *testing.T, components []*ComponentDetails) {
	// Unset variable for each component
	for _, component := range components {
		if len(component.EnvName) > 0 {
			zap.S().Infof("Unsetting environment variable %s", component.EnvName)
			err := os.Unsetenv(component.EnvName)
			assert.Nil(t, err, fmt.Sprintf("unsetting environment variable %s", component.EnvName))
		}
	}
}

func testImages(t *testing.T, components []*ComponentDetails, disabledComponents []*ComponentDetails) {
	err := os.Setenv(eswaitTargetVersionEnv, "es.TEST")
	assert.Nil(t, err, fmt.Sprintf("setting environment variable %s", eswaitTargetVersionEnv))

	err = InitComponentDetails()
	assert.Nil(t, err, "Expected initComponentDetails to succeed")

	// Test the image names were set as expected
	for _, component := range components {
		if len(component.EnvName) > 0 {
			assert.Equal(t, "TEST", component.Image, fmt.Sprintf("checking image name field for %s", component.Name))
		}
	}
	// Test the disabled status is set as expected
	for _, component := range disabledComponents {
		assert.True(t, component.Disabled, fmt.Sprintf("checking disabled status for %s", component.Name))
	}
}
