// Copyright (C) 2020, 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package vmo

import (
	"crypto/rand"
	"math/big"

	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// StorageClassInfo contains storage class info for PVC
type StorageClassInfo struct {
	Name              string
	PvcAcceptsZone    bool
	PvcZoneMatchLabel string
}

// Returns a list of ready and schedulable nodes
func getSchedulableNodes(controller *Controller) ([]corev1.Node, error) {
	var schedulableNodes []corev1.Node

	nodes, err := controller.nodeLister.List(labels.Everything())
	for _, node := range nodes {
		ready := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == constants.K8sReadyCondition && condition.Status == "True" {
				ready = true
			}
		}
		schedulable := true
		for _, taint := range node.Spec.Taints {
			if taint.Effect == constants.K8sTaintNoScheduleEffect {
				schedulable = false
			}
		}
		if ready && schedulable {
			schedulableNodes = append(schedulableNodes, *node)
		}
	}
	return schedulableNodes, err
}

// Returns a list of ADs which contain scheduable nodes
func getSchedulableADs(controller *Controller) ([]string, error) {
	var schedulableADs []string
	schedulableNodes, err := getSchedulableNodes(controller)
	if err != nil {
		return schedulableADs, err
	}
	for _, node := range schedulableNodes {
		ad, ok := node.Labels[constants.K8sZoneLabel]
		if ok {
			if !contains(schedulableADs, ad) {
				schedulableADs = append(schedulableADs, ad)
			}
		}
	}
	return schedulableADs, err
}
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Returns a random element from the given slice
func chooseRandomElementFromSlice(slice []string) string {
	if len(slice) > 0 {
		nBig, err := rand.Int(rand.Reader, big.NewInt(int64(len(slice))))
		if err != nil {
			panic("failed to generate random number")
		}
		return slice[nBig.Int64()]
	}
	return ""
}
