// Copyright (C) 2021, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package memory

import (
	"fmt"
)

// UnitK is 1 Kilobyte
const UnitK = 1024

// UnitM is 1 Megabyte
const UnitM = 1024 * UnitK

// UnitG is 1 Gigabyte
const UnitG = 1024 * UnitM

// FormatJvmHeapMinMax returns the identical min and max JVM heap setting in the format
// java expects like "-Xms2g -Xmx2g"
func FormatJvmHeapMinMax(heap string) string {
	return fmt.Sprintf("-Xms%s -Xmx%s", heap, heap)
}

// Format the string based on the size of the input value
// Return whole number (1200M not 1.2G)
// Return min magnatude of K
func FormatJvmHeapSize(sizeB int64) string {
	if sizeB >= UnitG {
		if sizeB%UnitG == 0 {
			// e.g 50g
			return fmt.Sprintf("%.0fg", float64(sizeB)/UnitG)
		}
		if sizeB%UnitM == 0 {
			// e.g. 1500m - value in exact mb
			return fmt.Sprintf("%.0fm", float64(sizeB)/UnitM)
		}
		// round up to next mb
		return fmt.Sprintf("%.0fm", float64(sizeB)/UnitM+1)
	}
	if sizeB >= UnitM && sizeB%UnitM == 0 {
		// e.g. 50m
		return fmt.Sprintf("%.0fm", float64(sizeB)/UnitM)
	}
	if sizeB%UnitK == 0 {
		// e.g. 1500k - value in exact kb
		return fmt.Sprintf("%.0fk", float64(sizeB)/UnitK)
	}
	// round up to next kb
	return fmt.Sprintf("%vk", sizeB/UnitK+1)
}

//func FormatJvmHeapSize(sizeB int64) string {
//	if sizeB >= UnitG && sizeB%UnitG == 0 {
//		return fmt.Sprintf("%.0fg", float64(sizeB)/UnitG)
//	}
//	if sizeB >= UnitM && sizeB%UnitM == 0 {
//		return fmt.Sprintf("%.0fm", float64(sizeB)/UnitM)
//	}
//	if sizeB%UnitK == 0 {
//		return fmt.Sprintf("%.0fk", float64(sizeB)/UnitK)
//	}
//
//	// Round up to next highest K
//	return fmt.Sprintf("%vk", (sizeB + UnitK)/UnitK)
//}
