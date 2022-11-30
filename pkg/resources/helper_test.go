// Copyright (C) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

package resources

import (
	"fmt"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	vmov1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
)

func createTestVMI() *vmov1.VerrazzanoMonitoringInstance {
	return &vmov1.VerrazzanoMonitoringInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system",
			Namespace: "test",
		},
	}
}

func TestGetOpenSearchDashboardsHTTPEndpoint(t *testing.T) {
	osdEndpoint := GetOpenSearchDashboardsHTTPEndpoint(createTestVMI())
	assert.Equal(t, "http://vmi-system-osd.test.svc.cluster.local:5601", osdEndpoint)
}

func TestGetOpenSearchHTTPEndpoint(t *testing.T) {
	osEndpoint := GetOpenSearchHTTPEndpoint(createTestVMI())
	assert.Equal(t, "http://vmi-system-es-master-http.test.svc.cluster.local:9200", osEndpoint)
}

func TestConvertToRegexp(t *testing.T) {
	var tests = []struct {
		pattern string
		regexp  string
	}{
		{
			"verrazzano-*",
			"^verrazzano-.*$",
		},
		{
			"verrazzano-system",
			"^verrazzano-system$",
		},
		{
			"*",
			"^.*$",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("converting pattern '%s' to regexp", tt.pattern), func(t *testing.T) {
			r := ConvertToRegexp(tt.pattern)
			assert.Equal(t, tt.regexp, r)
		})
	}
}

func TestGetCompLabel(t *testing.T) {
	var tests = []struct {
		compName     string
		expectedName string
	}{
		{
			"es-master",
			"opensearch",
		},
		{
			"es-data",
			"opensearch",
		},
		{
			"es-ingest",
			"opensearch",
		},
		{
			"foo",
			"foo",
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("component name '%s' to expectedName '%s'", tt.compName, tt.expectedName), func(t *testing.T) {
			r := GetCompLabel(tt.compName)
			assert.Equal(t, tt.expectedName, r)
		})
	}
}

func TestDeepCopyMap(t *testing.T) {
	var tests = []struct {
		srcMap map[string]string
		dstMap map[string]string
	}{
		{
			map[string]string{"foo": "bar"},
			map[string]string{"foo": "bar"},
		},
	}

	for _, tt := range tests {
		t.Run("basic deepcopy test", func(t *testing.T) {
			r := DeepCopyMap(tt.srcMap)
			assert.Equal(t, tt.dstMap, r)
		})
	}
}

// GIVEN a string representing java options settings for an OpenSerach container
// WHEN  CreateOpenSearchContainerCMD is invoked to get the command for the OpenSearch container
// THEN the command contains a subcommand to disable the jvm heap settings, if input contains java heap settings
func TestCreateOpenSearchContainerCMD(t *testing.T) {
	containerCmdWithoutJavaOpts := fmt.Sprintf(containerCmdTmpl, "", "")
	containerCmdWithJavaOpts := fmt.Sprintf(containerCmdTmpl, jvmOptsDisableCmd, "")
	var tests = []struct {
		description    string
		javaOpts       string
		expectedResult string
	}{
		{
			"testCreateOpenSearchContainerCMD with empty jvmOpts",
			"",
			containerCmdWithoutJavaOpts,
		},
		{
			"testCreateOpenSearchContainerCMD with jvmOpts not containing jvm memory settings",
			"-Xsomething",
			containerCmdWithoutJavaOpts,
		},
		{
			"testCreateOpenSearchContainerCMD with jvmOpts containing jvm memory settings",
			"-Xms1g -Xmx2g",
			containerCmdWithJavaOpts,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			r := CreateOpenSearchContainerCMD(tt.javaOpts, []string{})
			assert.Equal(t, tt.expectedResult, r)
		})
	}
}

// TestGetOpenSearchPluginList tests the GetOpenSearchPluginList
// GIVEN VMI CRD
// WHEN GetOpenSearchPluginList is called
// THEN returns the list of given OS plugins if there are plugins in VMI crd for OS, else empty list is returned
func TestGetOpenSearchPluginList(t *testing.T) {
	testPlugins := []string{"testPluginURL"}
	tests := []struct {
		name string
		vmo  *vmov1.VerrazzanoMonitoringInstance
		want []string
	}{
		{
			"TestGetOpenSearchPluginList when plugins are provided in VMI CRD",
			&vmov1.VerrazzanoMonitoringInstance{
				Spec: vmov1.VerrazzanoMonitoringInstanceSpec{
					Elasticsearch: vmov1.Elasticsearch{
						Enabled: true,
						Plugins: vmov1.Plugins{
							InstallList: testPlugins,
							Enabled:     true,
						},
					},
				},
			},
			testPlugins,
		},
		{
			"TestGetOpenSearchPluginList when plugins are not provided in VMI CRD",
			&vmov1.VerrazzanoMonitoringInstance{
				Spec: vmov1.VerrazzanoMonitoringInstanceSpec{
					Elasticsearch: vmov1.Elasticsearch{
						Enabled: true,
						Plugins: vmov1.Plugins{
							Enabled: false,
						},
					},
				},
			},
			[]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOpenSearchPluginList(tt.vmo); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetOpenSearchPluginList() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetOSDashboardPluginList tests the GetOSDashboardPluginList
// GIVEN VMI CRD
// WHEN GetOSDashboardPluginList is called
// THEN returns the list of given OSD plugins if there are plugins provided in VMI crd for OSD, else empty list is returned
func TestGetOSDashboardPluginList(t *testing.T) {
	testPlugins := []string{"testOSDPluginURL"}
	tests := []struct {
		name string
		vmo  *vmov1.VerrazzanoMonitoringInstance
		want []string
	}{
		{
			"TestGetOSDashboardPluginList when plugins are provided in VMI CRD",
			&vmov1.VerrazzanoMonitoringInstance{
				Spec: vmov1.VerrazzanoMonitoringInstanceSpec{
					Kibana: vmov1.Kibana{
						Enabled: true,
						Plugins: vmov1.Plugins{
							InstallList: testPlugins,
							Enabled:     true,
						},
					},
				},
			},
			testPlugins,
		},
		{
			"TestGetOSDashboardPluginList when plugins are not provided in VMI CRD",
			&vmov1.VerrazzanoMonitoringInstance{
				Spec: vmov1.VerrazzanoMonitoringInstanceSpec{
					Kibana: vmov1.Kibana{
						Enabled: true,
						Plugins: vmov1.Plugins{
							Enabled: false,
						},
					},
				},
			},
			[]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOSDashboardPluginList(tt.vmo); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetOSDashboardPluginList() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetOSPluginsInstallTmpl tests GetOSPluginsInstallTmpl
// GIVEN list of plugins name, URLs to plugins zip file or Maven coordinates.
// WHEN GetOSPluginsInstallTmpl is called
// THEN template is returned with updated plugins urls
func TestGetOSPluginsInstallTmpl(t *testing.T) {
	plugin := "testPluginsURL"
	tests := []struct {
		name    string
		plugins []string
		want    string
	}{
		{
			"TestGetOSPluginsInstallTmpl when list of plugins is provided",
			[]string{plugin},
			fmt.Sprintf(OSPluginsInstallTmpl, fmt.Sprintf(OSPluginsInstallCmd, plugin)),
		},
		{
			"TestGetOSPluginsInstallTmpl when no plugin is provided",
			[]string{},
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOSPluginsInstallTmpl(tt.plugins, OSPluginsInstallCmd); got != tt.want {
				t.Errorf("GetOSPluginsInstallTmpl() = %v, want %v", got, tt.want)
			}
		})
	}
}
