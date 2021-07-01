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
	asserts.Equal("-Xms375m -Xmx375m", s, "incorrect JVM heap arg")
	s, err = PodMemToJvmHeapArgs("1.4Gi")
	asserts.NoError(err, "error converting pod memory to JVM heap arg")
	asserts.Equal("-Xms1076m -Xmx1076m", s, "incorrect JVM heap arg")
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
		{name: "test-K-1", size: "100Ki", expected: "75k"},
		{name: "test-K-3", size: "1500Ki", expected: "1125k"},
		{name: "test-K-4", size: "1024Ki", expected: "768k"},
		{name: "test-K-5", size: ".5Ki", expected: "1k"},

		{name: "test-M-1", size: "1Mi", expected: "768k"},
		{name: "test-M-2", size: "1.1Mi", expected: "845k"},
		{name: "test-M-3", size: "1500Mi", expected: "1125m"},
		{name: "test-M-4", size: "1024Mi", expected: "768m"},
		{name: "test-M-5", size: ".5Mi", expected: "384k"},

		{name: "test-G-1", size: "1Gi", expected: "768m"},
		{name: "test-G-2", size: "1.1Gi", expected: "865076k"},
		{name: "test-G-3", size: "1500Gi", expected: "1125g"},
		{name: "test-G-4", size: "1024Gi", expected: "768g"},
		{name: "test-G-5", size: ".5Gi", expected: "384m"},
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
