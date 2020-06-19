// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package diff

import (
	"bufio"
	"github.com/kylelemons/godebug/pretty"
	"strings"
)

//
// Diffs two Golang objects recursively, but treats any elements whose values are empty in the 'desiredObject' as
// "no diff".  This is useful in a particular situation of comparing Kubernetes objects:
// 1) The 'desiredObject' is constructed via code.  It will almost certainly have many, many (nested) empty values.
// 2) It is compared against a 'liveObject', which has been retrieved live from the Kubernetes API, and has many of
//    these empty values populated with (k8s-generated) default values.
// In this situation, to determine whether our desiredObject is truly different than the liveObject, we ignore processing
// of elements whose values are empty in the desiredObject.
//
func CompareIgnoreTargetEmpties(liveObject interface{}, desiredObject interface{}) string {
	return filterDiffsIgnoreEmpties(pretty.Compare(liveObject, desiredObject))
}

//
// Filters the diff output of pretty.Compare(), returning the "true" diff output.  The basic rules for
// our filtering are as follows:
// - If a change is made to an element such that its new value is an empty value, the change is disregarded.
// - If a change is made to a map or list such that it is replaced by an empty map or list, the change is disregarded.
// - If a field is removed entirely (either simple value or a struct), the change is disregarded.
//
func filterDiffsIgnoreEmpties(diffOutput string) string {
	trueDiffs := ""
	scanner := bufio.NewScanner(strings.NewReader(diffOutput))

	// Track whether any true diffs are processed - otherwise we'll return an empty string
	foundDiffs := false
	var removalQueue []lineElement

	// We keep track of what nesting level we're at, and how many children exist within a given level.  This is necessary to
	// know whether the changes in diffOutput would make a map/list empty (which we disallow) when we process the end
	// of a map/list.  Also, along the way, we need to keep track of mutli-line structs that are candidates for (possible) deletion.
	currDepth := 0
	childrenPerDepth := map[int]int{0: 0}
	currStructDeleteDepth := -1 // -1 indicates that we are not currently in "struct deletion mode"
	var currStructDeleteLines []string
	currStructDeleteIsNamed := false
	numStructsDeleted := 0

	for scanner.Scan() {
		lineElem := parseLineElement(scanner.Text())

		// When a list or map starts, we increment the current depth and the number of children in the previous depth
		if lineElem.isStartOfMapOrList {
			childrenPerDepth[currDepth]++
			currDepth++
		}

		// Process reaching the end of a map or list
		if lineElem.isEndOfMapOrList {
			// When we pop out of the depth where struct deletion was in progress, process those struct deletions
			if currDepth == currStructDeleteDepth {
				// 2 cases where we don't allow struct removal to enter our final trueDiffs: 1) a named
				// struct is being completely removed 2) the struct(s) being removed results in an empty
				// parent slice/map.
				if !currStructDeleteIsNamed && childrenPerDepth[currDepth]-numStructsDeleted > 0 {
					trueDiffs += strings.Join(currStructDeleteLines, "\n") + "\n"
					currStructDeleteLines = []string{}
					foundDiffs = true
				}
				currStructDeleteDepth = -1
				numStructsDeleted = 0
				currStructDeleteIsNamed = false
			}
			// Pop out one level of depth
			childrenPerDepth[currDepth] = 0
			currDepth--
		}

		// Process the removal of an element
		if lineElem.isRemoval {
			// If we're removing a struct, we either initiate a new "struct deletion", or append to the
			// in-progress struct deletion (if we are at the same level as the in-progress struct deletion).
			if lineElem.isStartOfMapOrList {
				if currStructDeleteDepth < 0 { // Initate new struct deletion
					currStructDeleteDepth = currDepth - 1
					currStructDeleteIsNamed = lineElem.name != ""
				}
				if currStructDeleteDepth == currDepth-1 { // Append to the in-progress struct deletion
					numStructsDeleted++
				}
			}

			// We add the item to the normal removal queue if a struct deletion is not in progress
			if currStructDeleteDepth < 0 {
				// Removals of value-only elements are simply passed through as regular removals
				if lineElem.isValueOnly {
					trueDiffs += lineElem.fullLine + "\n"
					foundDiffs = true
				} else {
					removalQueue = append(removalQueue, lineElem)
				}
			} else {
				currStructDeleteLines = append(currStructDeleteLines, lineElem.fullLine)
			}
		}

		// Process the addition of an element
		if lineElem.isAddition {
			// We attempt to match the newly added element to a corresponding element sitting in the removal queue. Drain the removal queue
			// until it's either empty or we've found the element we're trying to match.
			for len(removalQueue) > 0 && removalQueue[0].name != lineElem.name {
				removalElement := removalQueue[0]
				removalQueue = removalQueue[1:]
				// Any unmatched removals are disregarded - the element is added back to our final trueDiffs without the "-".
				trueDiffs += " " + removalElement.fullLine[1:] + "\n"
			}

			// If no corresponding removal element was found, then simply add this addition element to our final trueDiffs
			if len(removalQueue) == 0 && !isValueEmpty(lineElem.value) {
				foundDiffs = true
				trueDiffs += lineElem.fullLine + "\n"
			} else if len(removalQueue) > 0 && lineElem.name == removalQueue[0].name {
				// Pop the matching element from the removal queue
				removalElement := removalQueue[0]
				removalQueue = removalQueue[1:]
				// We purposely ignore addition elements with null values!
				if !isValueEmpty(lineElem.value) {
					trueDiffs += removalElement.fullLine + "\n"
					trueDiffs += lineElem.fullLine + "\n"
					foundDiffs = true
				} else {
					// Add the _previous_ value for the removal element (with no removal specified) to our final trueDiffs
					trueDiffs += " " + removalElement.fullLine[1:] + "\n"
				}
			}
		}

		// Process a line without an addition/deletion operation
		if !lineElem.isAddition && !lineElem.isRemoval {
			// Drain the removal queue at this point if there are still any unprocessed/unmatched removals.  Any
			// such removals are disregarded - the element is added back to our final trueDiffs without the "-".
			for _, removeElement := range removalQueue {
				trueDiffs += " " + removeElement.fullLine[1:] + "\n"
			}
			removalQueue = []lineElement{}
			trueDiffs += lineElem.fullLine + "\n"
		}
	}

	if foundDiffs {
		return trueDiffs
	}
	return ""
}

// Whether the given value represents a logical empty value
func isValueEmpty(value string) bool {
	return value == "\"\"" || value == "0" || value == "0001-01-01 00:00:00 +0000 UTC" || value == "nil"
}

// Parses a given line of diff output into a 'lineElement', which contain the details of the line needed
// by the filterDiffsIgnoreEmpties() function.
func parseLineElement(line string) lineElement {
	lineElem := lineElement{fullLine: line}
	if line == "" {
		return lineElem
	}

	if strings.HasSuffix(line, "{") || strings.HasSuffix(line, "[") {
		lineElem.isStartOfMapOrList = true
	} else if strings.HasSuffix(line, "},") || strings.HasSuffix(line, "],") {
		lineElem.isEndOfMapOrList = true
	}

	if strings.HasPrefix(line, "-") {
		lineElem.isRemoval = true
	} else if strings.HasPrefix(line, "+") {
		lineElem.isAddition = true
	}

	// Parse tokens of the line string for name and value, if specified
	slice := strings.Fields(line[1:])
	if len(slice) > 1 {
		lineElem.name = strings.Trim(slice[0], ":")
		lineElem.value = line[strings.Index(line, slice[0])+len(slice[0]):]
		lineElem.value = strings.Trim(strings.TrimSpace(lineElem.value), ",")
	} else if !lineElem.isEndOfMapOrList && !lineElem.isStartOfMapOrList {
		lineElem.value = strings.Trim(strings.TrimSpace(line), ",")
		lineElem.isValueOnly = true
	}
	return lineElem
}

type lineElement struct {
	name               string
	value              string
	fullLine           string
	isStartOfMapOrList bool
	isEndOfMapOrList   bool
	isAddition         bool
	isRemoval          bool
	isValueOnly        bool
}
