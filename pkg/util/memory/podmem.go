// Copyright (C) 2021, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package memory

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// PodMemToJvmHeapArgs converts a pod resource memory to the JVM heap setting
// with same min/max, e.g: "-Xms512m -Xmx512m"
func PodMemToJvmHeapArgs(size, defaultValue string) (string, error) {
	if size == "" {
		return defaultValue, nil
	}

	s, err := PodMemToJvmHeap(size)
	if err != nil {
		return "", err
	}
	return FormatJvmHeapMinMax(s), nil
}

// PodMemToJvmHeap converts a pod resource memory (.5Gi) to the JVM heap setting
// in the format java expects (e.g. 512m)
func PodMemToJvmHeap(size string) (string, error) {
	q, err := resource.ParseQuantity(size)
	if err != nil {
		// This will never happen when VO creates the VMI since it formats the values correctly.
		// If someone happened to manually create a VMI this could happen if they format it wrong
		return "", err
	}
	// JVM setting should be around 75% of pod request size
	heap := (q.Value() * 75) / 100
	return FormatJvmHeapSize(heap), nil
}
