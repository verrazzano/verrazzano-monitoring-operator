// Copyright (C) 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package memory

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// TestFormatJvmHeapSize tests the formatting of values using 1024 multiples
// GIVEN a integer ranging from 100 to over 1G
// WHEN FormatJvmHeapSize is called
// THEN ensure the result has whole numbers and correct suffix is returned: K, M, or G suffix.
func TestFormatJvmHeapSize(t *testing.T) {
	asserts := assert.New(t)
	asserts.Equal("1k", FormatJvmHeapSize(100), "expected 1k")
	asserts.Equal("1k", FormatJvmHeapSize(UnitK), "expected 1k")
	asserts.Equal("10k", FormatJvmHeapSize(10*UnitK), "expected 10k")
	asserts.Equal("1000k", FormatJvmHeapSize(1000*UnitK), "expected 1000k")
	asserts.Equal("10000k", FormatJvmHeapSize(10000*UnitK), "expected 10000k")
	asserts.Equal("1m", FormatJvmHeapSize(UnitK*UnitK), "expected 1m")

	asserts.Equal("1m", FormatJvmHeapSize(UnitM), "expected 1m")
	asserts.Equal("10m", FormatJvmHeapSize(10*UnitM), "expected 10m")
	asserts.Equal("1000m", FormatJvmHeapSize(1000*UnitM), "expected 1000m")
	asserts.Equal("10000m", FormatJvmHeapSize(10000*UnitM), "expected 10000m")
	asserts.Equal("1g", FormatJvmHeapSize(UnitK*UnitM), "expected 1g")

	asserts.Equal("1g", FormatJvmHeapSize(UnitG), "expected 1g")
	asserts.Equal("10g", FormatJvmHeapSize(10*UnitG), "expected 10g")
	asserts.Equal("1000g", FormatJvmHeapSize(1000*UnitG), "expected 1000g")
	asserts.Equal("10000g", FormatJvmHeapSize(10000*UnitG), "expected 10000g")
	asserts.Equal("1024g", FormatJvmHeapSize(UnitK*UnitG), "expected 1024g")
}

// TestFormatJvmHeapMinMax tests the formatting of JVM heap string
// GIVEN a heap size
// WHEN FormatJvmHeapMinMax is called
// THEN ensure the result is a JVM formatted heap string with identical min/max heaps
func TestFormatJvmHeapMinMax(t *testing.T) {
	asserts := assert.New(t)
	asserts.Equal("-Xms500m -Xmx500m", FormatJvmHeapMinMax("500m"), "incorrect JVM heap string")
	asserts.Equal("-Xms2g -Xmx2g", FormatJvmHeapMinMax("2g"), "incorrect JVM heap string")
}
