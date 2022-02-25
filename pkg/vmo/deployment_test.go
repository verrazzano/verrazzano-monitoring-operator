// Copyright (C) 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"github.com/stretchr/testify/assert"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"testing"
)

func TestGetUpdateStrategy(t *testing.T) {
	var tests = []struct {
		name     string
		old      string
		new      string
		expected appsv1.DeploymentStrategyType
	}{
		{
			"recreate update",
			"foo",
			"bar",
			appsv1.RecreateDeploymentStrategyType,
		},
		{
			"rolling update",
			"foo",
			"foo",
			appsv1.RollingUpdateDeploymentStrategyType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategyType := getUpdateStrategy(tt.new, tt.old)
			assert.Equal(t, tt.expected, strategyType)
		})
	}
}

func TestAddKibanaUpgradeStrategy(t *testing.T) {
	newDeploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  config.Kibana.Name,
							Image: "foo",
						},
					},
				},
			},
		},
	}
	oldDeploy := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  config.Kibana.Name,
							Image: "bar",
						},
					},
				},
			},
		},
	}
	addKibanaUpgradeStrategy(newDeploy, oldDeploy)
	assert.Equal(t, appsv1.RecreateDeploymentStrategyType, newDeploy.Spec.Strategy.Type)
}
