// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package main

import "testing"

func TestIsSufficientNilInputs(t *testing.T) {
	if isSufficient(nil, nil, "7.5.0", 1) {
		t.Fail()
	}
}

func TestIsSufficientWrongVersion(t *testing.T) {
	clusterHealth := ClusterHealth{"yellow"}
	nodes := []ESNode{{Version: "7.4.0", Role: "d"}, {Version: "7.4.0", Role: "m"}, {Version: "7.4.0", Role: "i"}}
	if isSufficient(&clusterHealth, &nodes, "7.5.0", 1) {
		t.Fail()
	}
}

func TestIsSufficientOneOlderVersion(t *testing.T) {
	clusterHealth := ClusterHealth{"yellow"}
	nodes := []ESNode{{Version: "7.5.0", Role: "d"}, {Version: "7.5.0", Role: "m"}, {Version: "7.4.0", Role: "i"}}
	if isSufficient(&clusterHealth, &nodes, "7.5.0", 1) {
		t.Fail()
	}
}

func TestIsSufficientWrongStatus(t *testing.T) {
	clusterHealth := ClusterHealth{"red"}
	nodes := []ESNode{{Version: "7.5.0", Role: "d"}, {Version: "7.5.0", Role: "m"}, {Version: "7.5.0", Role: "i"}}
	if isSufficient(&clusterHealth, &nodes, "7.5.0", 1) {
		t.Fail()
	}
}

func TestIsSufficientNotEnoughDataNodes(t *testing.T) {
	clusterHealth := ClusterHealth{"yellow"}
	nodes := []ESNode{{Version: "7.5.0", Role: "d"}, {Version: "7.5.0", Role: "m"}, {Version: "7.5.0", Role: "i"}}
	if isSufficient(&clusterHealth, &nodes, "7.5.0", 2) {
		t.Fail()
	}
}

func TestIsSufficientNotEnoughDataNodesAtExpectedVersion(t *testing.T) {
	clusterHealth := ClusterHealth{"yellow"}
	nodes := []ESNode{{Version: "7.4.0", Role: "d"}, {Version: "7.5.0", Role: "m"}, {Version: "7.5.0", Role: "i"}}
	if isSufficient(&clusterHealth, &nodes, "7.5.0", 1) {
		t.Fail()
	}
}

func TestIsSufficient(t *testing.T) {
	clusterHealth := ClusterHealth{"yellow"}
	nodes := []ESNode{{Version: "7.5.0", Role: "d"}, {Version: "7.5.0", Role: "m"}, {Version: "7.5.0", Role: "i"}}
	if !isSufficient(&clusterHealth, &nodes, "7.5.0", 1) {
		t.Fail()
	}
}
