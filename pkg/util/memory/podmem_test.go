// Copyright (C) 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package memory

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestPodMemToJvmHeapArgs tests the conversion of Pod Mem to of JVM heap arg
// GIVEN a pod memory request, like .5Gi
// WHEN PodMemToJvmHeapArgs is called
// THEN ensure the result is a JVM formatted heap string like "-Xms512m -Xmx512m"
func TestPodMemToJvmHeapArgs(t *testing.T) {
	asserts := assert.New(t)
	s, err := PodMemToJvmHeapArgs("500Mi")
	asserts.NoError(err, "error converting pod memory to JVM heap arg")
	asserts.Equal("-Xms500m -Xmx500m", s, "incorrect JVM heap arg")
	s, err = PodMemToJvmHeapArgs("1.4Gi")
	asserts.NoError(err, "error converting pod memory to JVM heap arg")
	asserts.Equal("-Xms1435m -Xmx1435m", s, "incorrect JVM heap arg")
}

// TestPodMemToJvmHeap tests the formatting of pod memory requests into JVM heap sizes
// GIVEN a pod memory request, like .5Gi
// WHEN ConvertPodMemToJvmHeap is called
// THEN return the JVM formatted heap size, like 512m
func TestPodMemToJvmHeap(t *testing.T) {
	tests := []struct {
		name     string
		size     string
		expected string
	}{
		{name: "test-K-1", size: "1Ki", expected: "1k"},
		{name: "test-K-2", size: "1.1Ki", expected: "2k"},
		{name: "test-K-3", size: "1500Ki", expected: "1500k"},
		{name: "test-K-4", size: "1024Ki", expected: "1m"},
		{name: "test-K-5", size: ".5Ki", expected: "1k"},

		{name: "test-M-1", size: "1Mi", expected: "1m"},
		{name: "test-M-2", size: "1.1Mi", expected: "1127k"},
		{name: "test-M-3", size: "1500Mi", expected: "1500m"},
		{name: "test-M-4", size: "1024Mi", expected: "1g"},
		{name: "test-M-5", size: ".5Mi", expected: "512k"},

		{name: "test-G-1", size: "1Gi", expected: "1g"},
		{name: "test-G-2", size: "1.1Gi", expected: "1127m"},
		{name: "test-G-3", size: "1500Gi", expected: "1500g"},
		{name: "test-G-4", size: "1024Gi", expected: "1024g"},
		{name: "test-G-5", size: ".5Gi", expected: "512m"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			asserts := assert.New(t)
			n, err := PodMemToJvmHeap(test.size)
			asserts.NoError(err, "GetMaxHeap returned error")
			asserts.Equal(test.expected, n, fmt.Sprintf("%s failed", test.name))
		})
	}
}
