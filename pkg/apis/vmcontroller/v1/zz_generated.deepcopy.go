//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Copyright (c) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1

import (
	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *API) DeepCopyInto(out *API) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new API.
func (in *API) DeepCopy() *API {
	if in == nil {
		return nil
	}
	out := new(API)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertManager) DeepCopyInto(out *AlertManager) {
	*out = *in
	out.Resources = in.Resources
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertManager.
func (in *AlertManager) DeepCopy() *AlertManager {
	if in == nil {
		return nil
	}
	out := new(AlertManager)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerConfig) DeepCopyInto(out *ContainerConfig) {
	*out = *in
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.EnvFrom != nil {
		in, out := &in.EnvFrom, &out.EnvFrom
		*out = make([]corev1.EnvFromSource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerConfig.
func (in *ContainerConfig) DeepCopy() *ContainerConfig {
	if in == nil {
		return nil
	}
	out := new(ContainerConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ContainerSpec) DeepCopyInto(out *ContainerSpec) {
	*out = *in
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.EnvFrom != nil {
		in, out := &in.EnvFrom, &out.EnvFrom
		*out = make([]corev1.EnvFromSource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Volumes != nil {
		in, out := &in.Volumes, &out.Volumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.VolumeMounts != nil {
		in, out := &in.VolumeMounts, &out.VolumeMounts
		*out = make([]corev1.VolumeMount, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ContainerSpec.
func (in *ContainerSpec) DeepCopy() *ContainerSpec {
	if in == nil {
		return nil
	}
	out := new(ContainerSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Database) DeepCopyInto(out *Database) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Database.
func (in *Database) DeepCopy() *Database {
	if in == nil {
		return nil
	}
	out := new(Database)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Elasticsearch) DeepCopyInto(out *Elasticsearch) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
	in.IngestNode.DeepCopyInto(&out.IngestNode)
	in.MasterNode.DeepCopyInto(&out.MasterNode)
	in.DataNode.DeepCopyInto(&out.DataNode)
	if in.Policies != nil {
		in, out := &in.Policies, &out.Policies
		*out = make([]IndexManagementPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Nodes != nil {
		in, out := &in.Nodes, &out.Nodes
		*out = make([]ElasticsearchNode, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.Plugins.DeepCopyInto(&out.Plugins)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Elasticsearch.
func (in *Elasticsearch) DeepCopy() *Elasticsearch {
	if in == nil {
		return nil
	}
	out := new(Elasticsearch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ElasticsearchNode) DeepCopyInto(out *ElasticsearchNode) {
	*out = *in
	out.Resources = in.Resources
	if in.Storage != nil {
		in, out := &in.Storage, &out.Storage
		*out = new(Storage)
		(*in).DeepCopyInto(*out)
	}
	if in.Roles != nil {
		in, out := &in.Roles, &out.Roles
		*out = make([]NodeRole, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ElasticsearchNode.
func (in *ElasticsearchNode) DeepCopy() *ElasticsearchNode {
	if in == nil {
		return nil
	}
	out := new(ElasticsearchNode)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Grafana) DeepCopyInto(out *Grafana) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
	out.Resources = in.Resources
	if in.Database != nil {
		in, out := &in.Database, &out.Database
		*out = new(Database)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Grafana.
func (in *Grafana) DeepCopy() *Grafana {
	if in == nil {
		return nil
	}
	out := new(Grafana)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPSpec) DeepCopyInto(out *HTTPSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPSpec.
func (in *HTTPSpec) DeepCopy() *HTTPSpec {
	if in == nil {
		return nil
	}
	out := new(HTTPSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IndexManagementPolicy) DeepCopyInto(out *IndexManagementPolicy) {
	*out = *in
	if in.MinIndexAge != nil {
		in, out := &in.MinIndexAge, &out.MinIndexAge
		*out = new(string)
		**out = **in
	}
	in.Rollover.DeepCopyInto(&out.Rollover)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IndexManagementPolicy.
func (in *IndexManagementPolicy) DeepCopy() *IndexManagementPolicy {
	if in == nil {
		return nil
	}
	out := new(IndexManagementPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Kibana) DeepCopyInto(out *Kibana) {
	*out = *in
	out.Resources = in.Resources
	in.Plugins.DeepCopyInto(&out.Plugins)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Kibana.
func (in *Kibana) DeepCopy() *Kibana {
	if in == nil {
		return nil
	}
	out := new(Kibana)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Metric) DeepCopyInto(out *Metric) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Percentiles != nil {
		in, out := &in.Percentiles, &out.Percentiles
		*out = make([]float64, len(*in))
		copy(*out, *in)
	}
	if in.Buckets != nil {
		in, out := &in.Buckets, &out.Buckets
		*out = make([]float64, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Metric.
func (in *Metric) DeepCopy() *Metric {
	if in == nil {
		return nil
	}
	out := new(Metric)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenSearchDashboardsPlugins) DeepCopyInto(out *OpenSearchDashboardsPlugins) {
	*out = *in
	if in.InstallList != nil {
		in, out := &in.InstallList, &out.InstallList
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenSearchDashboardsPlugins.
func (in *OpenSearchDashboardsPlugins) DeepCopy() *OpenSearchDashboardsPlugins {
	if in == nil {
		return nil
	}
	out := new(OpenSearchDashboardsPlugins)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *OpenSearchPlugins) DeepCopyInto(out *OpenSearchPlugins) {
	*out = *in
	if in.InstallList != nil {
		in, out := &in.InstallList, &out.InstallList
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new OpenSearchPlugins.
func (in *OpenSearchPlugins) DeepCopy() *OpenSearchPlugins {
	if in == nil {
		return nil
	}
	out := new(OpenSearchPlugins)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Plugins) DeepCopyInto(out *Plugins) {
	*out = *in
	if in.InstallList != nil {
		in, out := &in.InstallList, &out.InstallList
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Plugins.
func (in *Plugins) DeepCopy() *Plugins {
	if in == nil {
		return nil
	}
	out := new(Plugins)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Prometheus) DeepCopyInto(out *Prometheus) {
	*out = *in
	in.Storage.DeepCopyInto(&out.Storage)
	out.Resources = in.Resources
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Prometheus.
func (in *Prometheus) DeepCopy() *Prometheus {
	if in == nil {
		return nil
	}
	out := new(Prometheus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Resources) DeepCopyInto(out *Resources) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Resources.
func (in *Resources) DeepCopy() *Resources {
	if in == nil {
		return nil
	}
	out := new(Resources)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RolloverPolicy) DeepCopyInto(out *RolloverPolicy) {
	*out = *in
	if in.MinIndexAge != nil {
		in, out := &in.MinIndexAge, &out.MinIndexAge
		*out = new(string)
		**out = **in
	}
	if in.MinSize != nil {
		in, out := &in.MinSize, &out.MinSize
		*out = new(string)
		**out = **in
	}
	if in.MinDocCount != nil {
		in, out := &in.MinDocCount, &out.MinDocCount
		*out = new(int)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RolloverPolicy.
func (in *RolloverPolicy) DeepCopy() *RolloverPolicy {
	if in == nil {
		return nil
	}
	out := new(RolloverPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScriptConfig) DeepCopyInto(out *ScriptConfig) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScriptConfig.
func (in *ScriptConfig) DeepCopy() *ScriptConfig {
	if in == nil {
		return nil
	}
	out := new(ScriptConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScriptSpec) DeepCopyInto(out *ScriptSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScriptSpec.
func (in *ScriptSpec) DeepCopy() *ScriptSpec {
	if in == nil {
		return nil
	}
	out := new(ScriptSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Storage) DeepCopyInto(out *Storage) {
	*out = *in
	if in.PvcNames != nil {
		in, out := &in.PvcNames, &out.PvcNames
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Storage.
func (in *Storage) DeepCopy() *Storage {
	if in == nil {
		return nil
	}
	out := new(Storage)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VerrazzanoMonitoringInstance) DeepCopyInto(out *VerrazzanoMonitoringInstance) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VerrazzanoMonitoringInstance.
func (in *VerrazzanoMonitoringInstance) DeepCopy() *VerrazzanoMonitoringInstance {
	if in == nil {
		return nil
	}
	out := new(VerrazzanoMonitoringInstance)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VerrazzanoMonitoringInstance) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VerrazzanoMonitoringInstanceList) DeepCopyInto(out *VerrazzanoMonitoringInstanceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]VerrazzanoMonitoringInstance, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VerrazzanoMonitoringInstanceList.
func (in *VerrazzanoMonitoringInstanceList) DeepCopy() *VerrazzanoMonitoringInstanceList {
	if in == nil {
		return nil
	}
	out := new(VerrazzanoMonitoringInstanceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *VerrazzanoMonitoringInstanceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VerrazzanoMonitoringInstanceSpec) DeepCopyInto(out *VerrazzanoMonitoringInstanceSpec) {
	*out = *in
	out.Versioning = in.Versioning
	if in.IngressClassName != nil {
		in, out := &in.IngressClassName, &out.IngressClassName
		*out = new(string)
		**out = **in
	}
	in.Grafana.DeepCopyInto(&out.Grafana)
	in.Prometheus.DeepCopyInto(&out.Prometheus)
	out.AlertManager = in.AlertManager
	in.Elasticsearch.DeepCopyInto(&out.Elasticsearch)
	in.Kibana.DeepCopyInto(&out.Kibana)
	out.API = in.API
	if in.NatGatewayIPs != nil {
		in, out := &in.NatGatewayIPs, &out.NatGatewayIPs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.StorageClass != nil {
		in, out := &in.StorageClass, &out.StorageClass
		*out = new(string)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VerrazzanoMonitoringInstanceSpec.
func (in *VerrazzanoMonitoringInstanceSpec) DeepCopy() *VerrazzanoMonitoringInstanceSpec {
	if in == nil {
		return nil
	}
	out := new(VerrazzanoMonitoringInstanceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VerrazzanoMonitoringInstanceStatus) DeepCopyInto(out *VerrazzanoMonitoringInstanceStatus) {
	*out = *in
	if in.CreationTime != nil {
		in, out := &in.CreationTime, &out.CreationTime
		*out = (*in).DeepCopy()
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VerrazzanoMonitoringInstanceStatus.
func (in *VerrazzanoMonitoringInstanceStatus) DeepCopy() *VerrazzanoMonitoringInstanceStatus {
	if in == nil {
		return nil
	}
	out := new(VerrazzanoMonitoringInstanceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Versioning) DeepCopyInto(out *Versioning) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Versioning.
func (in *Versioning) DeepCopy() *Versioning {
	if in == nil {
		return nil
	}
	out := new(Versioning)
	in.DeepCopyInto(out)
	return out
}
