// Copyright (C) 2020, Oracle Corporation and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package diff

import (
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	"github.com/verrazzano/verrazzano-monitoring-operator/pkg/resources"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"testing"
)

func verifyFilterDiffs(t *testing.T, input string, expected string, description string) {
	assert.Equal(t, expected, filterDiffsIgnoreEmpties(input), description)
}

// Examples of parsing a single line of diff output using the parseLineElement() function
func TestParseElement(t *testing.T) {
	input := ""
	result := parseLineElement(input)
	expected := lineElement{}
	assert.Equal(t, expected, result, "")

	// Examples of a simple value - plain, removal, and addition
	result = parseLineElement("  Foo: 123")
	expected = lineElement{fullLine: "  Foo: 123", name: "Foo", value: "123"}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("- Foo: 123")
	expected = lineElement{fullLine: "- Foo: 123", name: "Foo", value: "123", isRemoval: true}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("+ Foo: 123")
	expected = lineElement{fullLine: "+ Foo: 123", name: "Foo", value: "123", isAddition: true}
	assert.Equal(t, expected, result, "")

	// Examples of a value-only element
	result = parseLineElement("  Foo")
	expected = lineElement{fullLine: "  Foo", value: "Foo", isValueOnly: true}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("  \"Foo\"")
	expected = lineElement{fullLine: "  \"Foo\"", value: "\"Foo\"", isValueOnly: true}
	assert.Equal(t, expected, result, "")

	// Examples of the start of a named struct - plain, addition, and removal
	result = parseLineElement("  ObjectMeta: {")
	expected = lineElement{isStartOfMapOrList: true, fullLine: "  ObjectMeta: {", name: "ObjectMeta", value: "{"}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("- ObjectMeta: {")
	expected = lineElement{isStartOfMapOrList: true, fullLine: "- ObjectMeta: {", name: "ObjectMeta", value: "{", isRemoval: true}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("+ ObjectMeta: {")
	expected = lineElement{isStartOfMapOrList: true, fullLine: "+ ObjectMeta: {", name: "ObjectMeta", value: "{", isAddition: true}
	assert.Equal(t, expected, result, "")

	// Examples of the start of an unnamed struct
	result = parseLineElement("  {")
	expected = lineElement{isStartOfMapOrList: true, fullLine: "  {", name: "", value: ""}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("- {")
	expected = lineElement{isStartOfMapOrList: true, fullLine: "- {", name: "", value: "", isRemoval: true}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("+ {")
	expected = lineElement{isStartOfMapOrList: true, fullLine: "+ {", name: "", value: "", isAddition: true}
	assert.Equal(t, expected, result, "")

	// Examples of the end of a struct
	result = parseLineElement("  },")
	expected = lineElement{isEndOfMapOrList: true, fullLine: "  },", name: "", value: ""}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("- },")
	expected = lineElement{isEndOfMapOrList: true, fullLine: "- },", name: "", value: "", isRemoval: true}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("+ },")
	expected = lineElement{isEndOfMapOrList: true, fullLine: "+ },", name: "", value: "", isAddition: true}
	assert.Equal(t, expected, result, "")

	// Examples of the end of a list
	result = parseLineElement("  ],")
	expected = lineElement{isEndOfMapOrList: true, fullLine: "  ],", name: "", value: ""}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("- ],")
	expected = lineElement{isEndOfMapOrList: true, fullLine: "- ],", name: "", value: "", isRemoval: true}
	assert.Equal(t, expected, result, "")
	result = parseLineElement("+ ],")
	expected = lineElement{isEndOfMapOrList: true, fullLine: "+ ],", name: "", value: "", isAddition: true}
	assert.Equal(t, expected, result, "")

	// A few others
	input = "-   deployment.kubernetes.io/revision: \"1\","
	result = parseLineElement(input)
	expected = lineElement{name: "deployment.kubernetes.io/revision", value: "\"1\"", fullLine: input, isRemoval: true}
	assert.Equal(t, expected, result, "")
}

func TestFilterDiffs01(t *testing.T) {
	input := `{
  ObjectMeta: {
   Name: "sauron-foo1-grafana",
   GenerateName: "",
   Namespace: "foo1",
-  SelfLink: "/apis/apps/v1/namespaces/foo1/deployments/sauron-foo1-grafana",
-  UID: "a90900b7-2a35-11e9-86dc-0a580aed2744",
-  ResourceVersion: "17752256",
-  Generation: 1,
+  SelfLink: "",
+  UID: "",
+  ResourceVersion: "",
+  Generation: 0,
   CreationTimestamp: {
-   Time: 2019-02-06 09:35:56 -0800 PST,
+   Time: 0001-01-01 00:00:00 +0000 UTC,
   },
  },
 }
`
	verifyFilterDiffs(t, input, "", "Values updated with a variety of nils")
}

func TestFilterDiffs02(t *testing.T) {
	input := `{
  ObjectMeta: {
   Name: "sauron-foo1-grafana",
   GenerateName: "",
   Namespace: "foo1",
-  SelfLink: "/apis/apps/v1/namespaces/foo1/deployments/sauron-foo1-grafana",
-  UID: "a90900b7-2a35-11e9-86dc-0a580aed2744",
-  ResourceVersion: "17752256",
-  Generation: 1,
-  Generation1: 1,
-  Generation2: 1,
+  SelfLink: "",
+  UID: "",
+  ResourceVersion: "",
+  Generation: 0,
+  Generation1: 2,
+  Generation2: nil,
   CreationTimestamp: {
-   Time: 2019-02-06 09:35:56 -0800 PST,
+   Time: 0001-01-01 00:00:00 +0000 UTC,
   },
   CreationTimestamp1: {
-   Time: 2019-02-06 09:35:56 -0800 PST,
+   Time: 9999-01-01 00:00:00 +0000 UTC,
   },
  },
 }
`
	expected := `{
  ObjectMeta: {
   Name: "sauron-foo1-grafana",
   GenerateName: "",
   Namespace: "foo1",
   SelfLink: "/apis/apps/v1/namespaces/foo1/deployments/sauron-foo1-grafana",
   UID: "a90900b7-2a35-11e9-86dc-0a580aed2744",
   ResourceVersion: "17752256",
   Generation: 1,
-  Generation1: 1,
+  Generation1: 2,
   Generation2: 1,
   CreationTimestamp: {
    Time: 2019-02-06 09:35:56 -0800 PST,
   },
   CreationTimestamp1: {
-   Time: 2019-02-06 09:35:56 -0800 PST,
+   Time: 9999-01-01 00:00:00 +0000 UTC,
   },
  },
 }
`
	verifyFilterDiffs(t, input, expected, "Values updated with a variety of nils and non-nils")
}

func TestFilterDiffs03(t *testing.T) {
	input := `{
  ObjectMeta: {
   Name: "sauron-foo1-grafana",
+  Foo1: "foo1",
+  Foo2: "foo2",
+  Foo3: "foo3",
+  Foo4: "foo4",
   Bar: {
+   Bar1 : "bar1"
    },
  },
 }
`
	verifyFilterDiffs(t, input, input, "Values added only")
}

func TestFilterDiffs04(t *testing.T) {
	input := `{
  ObjectMeta: {
   Name: "sauron-foo1-grafana",
-  Foo1: "foo1",
-  Foo2: "foo2",
-  Foo3: "foo3",
-  Foo4: "foo4",
   Bar: {
-   Bar1 : "bar1"
    },
  },
 }
`
	verifyFilterDiffs(t, input, "", "Values removed only")
}

func TestFilterDiffs05(t *testing.T) {
	input := `{
  ObjectMeta: {
+  Foo1: "foo1",
-  Foo2: "foo2",
-  Foo3: "foo3",
-  Foo4: "foo4",
-  Foo5: "foo5",
+  Foo3: "foo31",
+  Foo5: "foo51",
+  Foo6: "foo6",
-  Foo7: "foo7",
+  Foo7: "foo71",
-  Foo8: "foo6",
   Bar: {
    Baz: {
+    Bar1: "bar1",
-    Bar2: "bar2",
-    Bar3: "bar3",
-    Bar4: "bar4",
    },
+   Bar1: "bar1",
-   Bar2: "bar2",
-   Bar3: "bar3",
-   Bar4: "bar4",
-   Bar5: "bar5",
+   Bar3: "bar31",
+   Bar5: "bar51",
+   Bar6: "bar6",
-   Bar7: "bar7",
+   Bar7: "bar71",
-   Bar8: "bar8",
    },
  },
 }
`
	expected := `{
  ObjectMeta: {
+  Foo1: "foo1",
   Foo2: "foo2",
-  Foo3: "foo3",
+  Foo3: "foo31",
   Foo4: "foo4",
-  Foo5: "foo5",
+  Foo5: "foo51",
+  Foo6: "foo6",
-  Foo7: "foo7",
+  Foo7: "foo71",
   Foo8: "foo6",
   Bar: {
    Baz: {
+    Bar1: "bar1",
     Bar2: "bar2",
     Bar3: "bar3",
     Bar4: "bar4",
    },
+   Bar1: "bar1",
    Bar2: "bar2",
-   Bar3: "bar3",
+   Bar3: "bar31",
    Bar4: "bar4",
-   Bar5: "bar5",
+   Bar5: "bar51",
+   Bar6: "bar6",
-   Bar7: "bar7",
+   Bar7: "bar71",
    Bar8: "bar8",
    },
  },
 }
`
	verifyFilterDiffs(t, input, expected, "Sequence of adds/removals/updates, at different levels")
}

func TestFilterDiffs06(t *testing.T) {
	input := `{
  Spec: {
   Env: [
-   {
-    Foo: "foo3",
-    Bar: "bar3",
-   },
   ],
  },
 }
`
	verifyFilterDiffs(t, input, "", "Removal of the last lineElement of a slice")
}

func TestFilterDiffs07(t *testing.T) {
	input := `{
  Spec: {
   Env: [
-   {
-    Foo: "foo",
-    Bar: "bar",
-   },
    {
     Foo1: "foo1",
     Bar1: "bar1",
    },
   ],
  },
 }
`
	expected := `{
  Spec: {
   Env: [
    {
     Foo1: "foo1",
     Bar1: "bar1",
    },
-   {
-    Foo: "foo",
-    Bar: "bar",
-   },
   ],
  },
 }
`
	verifyFilterDiffs(t, input, expected, "Removal of the non-last lineElement of a slice")
}

func TestFilterDiffs08(t *testing.T) {
	input := `{
  Spec: {
   Env: [
-   {
-    Foo: "foo",
-    Bar: "bar",
-    {
-     Foo: "foo",
-     Bar: "bar",
-     {
-      Foo: "foo",
-      Bar: "bar",
-      {
-       Foo: "foo",
-       Bar: "bar",
-      },
-     },
-    },
-   },
   ],
-  Foo1: "foo",
-  Foo: "foo",
+  Foo: "foo1",
  },
 }
`
	expected := `{
  Spec: {
   Env: [
   ],
   Foo1: "foo",
-  Foo: "foo",
+  Foo: "foo1",
  },
 }
`
	verifyFilterDiffs(t, input, expected, "Removal of the last lineElement of a slice, requiring traversal of multiple levels")
}

func TestFilterDiffs09(t *testing.T) {
	input := `{
  Spec: {
   Env: [
-   {
-    Foo: "foo1",
-    Bar: "bar1",
-   },
+   {
+    Foo1: "foo2",
+    Bar1: "bar2",
+   },
    {
     Foo3: "foo3",
     Bar3: "bar3",
    },
-   {
-    Foo4: "foo4",
-    Bar4: "bar4",
-   },
   ],
  },
 }
`
	expected := `{
  Spec: {
   Env: [
+   {
+    Foo1: "foo2",
+    Bar1: "bar2",
+   },
    {
     Foo3: "foo3",
     Bar3: "bar3",
    },
-   {
-    Foo: "foo1",
-    Bar: "bar1",
-   },
-   {
-    Foo4: "foo4",
-    Bar4: "bar4",
-   },
   ],
  },
 }
`
	verifyFilterDiffs(t, input, expected, "Removal of the last lineElement of a map")
}

func TestFilterDiffs10(t *testing.T) {
	input := `{
  Spec: {
   Env: [
-   {
-    Foo: "foo1",
-    Bar: "bar1",
-   },
+   {
+    Foo1: "foo2",
+    Bar1: "bar2",
+   },
   ],
   Env1: [
-   {
-    Foo: "foo1",
-    Bar: "bar1",
-   },
   ],
  },
 }
`
	expected := `{
  Spec: {
   Env: [
+   {
+    Foo1: "foo2",
+    Bar1: "bar2",
+   },
-   {
-    Foo: "foo1",
-    Bar: "bar1",
-   },
   ],
   Env1: [
   ],
  },
 }
`
	verifyFilterDiffs(t, input, expected, "Removal of the last lineElement of a slice, after an early slice at the same level")
}

func TestFilterDiffs11(t *testing.T) {
	input := `{
  Template: {
-  SecurityContext: {
-   SELinuxOptions: nil,
-  },
   ImagePullSecrets: [
   ],
-  Foo: foo,
+  Foo: foo1,
 }
`
	expected := `{
  Template: {
   ImagePullSecrets: [
   ],
+  Foo: foo1,
 }
`
	verifyFilterDiffs(t, input, expected, "Removal of entire named struct element, shouldn't be removed in final result")
}

func TestFilterDiffsMalformed01(t *testing.T) {
	input := "abcdef"
	verifyFilterDiffs(t, input, "", "Malformed input")
}

func TestFilterDiffsMalformed02(t *testing.T) {
	input := `{
  Template: {
-  SecurityContext: {
`
	verifyFilterDiffs(t, input, "", "Malformed input")
}

func TestFilterDiffsMalformed03(t *testing.T) {
	input := `{
-  SecurityContext: {
 }
}
}
`
	verifyFilterDiffs(t, input, "", "Malformed input")
}

func TestFilterDiffs12(t *testing.T) {
	input := ` {
   PvcNames: [
-   "foo",
    "bar",
   ],
 }
`
	verifyFilterDiffs(t, input, input, "Removal of a value-only element")
}

func TestFilterDiffs13(t *testing.T) {
	input := ` {
   PvcNames: [
-   foo,
+   bar,
   ],
 }
`
	verifyFilterDiffs(t, input, input, "Removal of a value-only element")
}

func TestFilterDiffs14(t *testing.T) {
	input := `{
  Spec: {
-  Env: [
-    Foo,
-    Bar,
-  ],
  },
 }
`
	verifyFilterDiffs(t, input, "", "Removal of an entire list, including value-only elements")
}

// Examples of what this looks like on a real Deployment objects.  For brevity, we won't actually check the full diff output in
// these cases (it's long), just that the diffs are shown or not.
func TestCompare(t *testing.T) {
	// Empty deployments
	liveDeployment := appsv1.Deployment{}
	desiredDeployment := appsv1.Deployment{}
	assert.Equal(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Empty objects")

	// Simple values set in the live deployment and not in the desired
	liveDeployment.Name = "foo"
	liveDeployment.Spec.Replicas = resources.NewVal(5)
	assert.Equal(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Empty values in desired")

	// Simple values set in the desired deployment
	desiredDeployment.Name = "foo"
	desiredDeployment.Spec.Replicas = resources.NewVal(5)
	assert.Equal(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Same values in desired object as live")
	desiredDeployment.Spec.Replicas = resources.NewVal(6)
	assert.NotEqual(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Different value in desired object from live")
	desiredDeployment.Spec.Replicas = resources.NewVal(5)
	desiredDeployment.Spec.MinReadySeconds = 500
	assert.NotEqual(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Different value in desired object from live")
	desiredDeployment.Spec.MinReadySeconds = 0

	// List (containers) specified in live deployment, empty in desired
	liveContainer := corev1.Container{}
	liveContainer.Name = "bar"
	liveDeployment.Spec.Template.Spec.Containers = []corev1.Container{liveContainer}
	assert.Equal(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Empty list in desired object")
	desiredDeployment.Spec.Template.Spec.Containers = []corev1.Container{}
	assert.Equal(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Empty list in desired object")

	// Non-empty list (containers) in desired
	desiredContainer := corev1.Container{}
	desiredContainer.Name = "bar"
	desiredDeployment.Spec.Template.Spec.Containers = []corev1.Container{desiredContainer}
	assert.Equal(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Same values in desired object as live")
	desiredContainer.Name = "bar1"
	desiredDeployment.Spec.Template.Spec.Containers = []corev1.Container{desiredContainer}
	assert.NotEqual(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Different value in desired object from live")
	desiredContainer.Name = "bar"
	desiredDeployment.Spec.Template.Spec.Containers = []corev1.Container{desiredContainer, desiredContainer}
	assert.NotEqual(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Different sized list in desired object from live")

	// Same thing as the above, but go one level deeper into EnvVars for a Container
	// List (EnvVars) specified in live deployment, empty in desired
	liveContainer = corev1.Container{}
	liveContainer.Env = []corev1.EnvVar{{Name: "foo", Value: "bar"}}
	liveDeployment.Spec.Template.Spec.Containers = []corev1.Container{liveContainer}
	desiredContainer = corev1.Container{}
	desiredDeployment.Spec.Template.Spec.Containers = []corev1.Container{desiredContainer}
	assert.Equal(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Empty list in desired object")
	desiredContainer = corev1.Container{}
	desiredContainer.Env = []corev1.EnvVar{}
	desiredDeployment.Spec.Template.Spec.Containers = []corev1.Container{desiredContainer}
	assert.Equal(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Empty list in desired object")

	// Non-empty list (EnvVars) in desired
	desiredContainer = corev1.Container{}
	desiredContainer.Env = []corev1.EnvVar{{Name: "foo", Value: "bar"}}
	desiredDeployment.Spec.Template.Spec.Containers = []corev1.Container{desiredContainer}
	assert.Equal(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Same values in desired object as live")
	desiredContainer.Env = []corev1.EnvVar{{Name: "foo", Value: "bar1"}}
	desiredDeployment.Spec.Template.Spec.Containers = []corev1.Container{desiredContainer}
	assert.NotEqual(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Different value in desired object from live")
	desiredContainer.Env = []corev1.EnvVar{{Name: "foo", Value: "bar"}, {Name: "foo1", Value: "bar1"}}
	assert.NotEqual(t, "", CompareIgnoreTargetEmpties(liveDeployment, desiredDeployment), "Different sized list in desired object from live")
}

// Other examples of changes on the Sauron spec
func TestCompare1(t *testing.T) {
	// Reducing the PVCs list of a live Sauron - should get reported as a diff
	liveSauron := v1.VerrazzanoMonitoringInstance{}
	liveSauron.Spec.Elasticsearch.Storage.PvcNames = []string{"foo", "bar"}
	desiredSauron := v1.VerrazzanoMonitoringInstance{}
	desiredSauron.Spec.Elasticsearch.Storage.PvcNames = []string{"foo"}
	assert.NotEqual(t, "", CompareIgnoreTargetEmpties(liveSauron, desiredSauron), "Different sized PVC lists")
}
