// Copyright (c) 2020, 2022, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeVerrazzanoMonitoringInstances implements VerrazzanoMonitoringInstanceInterface
type FakeVerrazzanoMonitoringInstances struct {
	Fake *FakeVerrazzanoV1
	ns   string
}

var verrazzanomonitoringinstancesResource = schema.GroupVersionResource{Group: "verrazzano.io", Version: "v1", Resource: "verrazzanomonitoringinstances"}

var verrazzanomonitoringinstancesKind = schema.GroupVersionKind{Group: "verrazzano.io", Version: "v1", Kind: "VerrazzanoMonitoringInstance"}

// Get takes name of the verrazzanoMonitoringInstance, and returns the corresponding verrazzanoMonitoringInstance object, and an error if there is any.
func (c *FakeVerrazzanoMonitoringInstances) Get(ctx context.Context, name string, options v1.GetOptions) (result *vmcontrollerv1.VerrazzanoMonitoringInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(verrazzanomonitoringinstancesResource, c.ns, name), &vmcontrollerv1.VerrazzanoMonitoringInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*vmcontrollerv1.VerrazzanoMonitoringInstance), err
}

// List takes label and field selectors, and returns the list of VerrazzanoMonitoringInstances that match those selectors.
func (c *FakeVerrazzanoMonitoringInstances) List(ctx context.Context, opts v1.ListOptions) (result *vmcontrollerv1.VerrazzanoMonitoringInstanceList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(verrazzanomonitoringinstancesResource, verrazzanomonitoringinstancesKind, c.ns, opts), &vmcontrollerv1.VerrazzanoMonitoringInstanceList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &vmcontrollerv1.VerrazzanoMonitoringInstanceList{ListMeta: obj.(*vmcontrollerv1.VerrazzanoMonitoringInstanceList).ListMeta}
	for _, item := range obj.(*vmcontrollerv1.VerrazzanoMonitoringInstanceList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested verrazzanoMonitoringInstances.
func (c *FakeVerrazzanoMonitoringInstances) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(verrazzanomonitoringinstancesResource, c.ns, opts))

}

// Create takes the representation of a verrazzanoMonitoringInstance and creates it.  Returns the server's representation of the verrazzanoMonitoringInstance, and an error, if there is any.
func (c *FakeVerrazzanoMonitoringInstances) Create(ctx context.Context, verrazzanoMonitoringInstance *vmcontrollerv1.VerrazzanoMonitoringInstance, opts v1.CreateOptions) (result *vmcontrollerv1.VerrazzanoMonitoringInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(verrazzanomonitoringinstancesResource, c.ns, verrazzanoMonitoringInstance), &vmcontrollerv1.VerrazzanoMonitoringInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*vmcontrollerv1.VerrazzanoMonitoringInstance), err
}

// Update takes the representation of a verrazzanoMonitoringInstance and updates it. Returns the server's representation of the verrazzanoMonitoringInstance, and an error, if there is any.
func (c *FakeVerrazzanoMonitoringInstances) Update(ctx context.Context, verrazzanoMonitoringInstance *vmcontrollerv1.VerrazzanoMonitoringInstance, opts v1.UpdateOptions) (result *vmcontrollerv1.VerrazzanoMonitoringInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(verrazzanomonitoringinstancesResource, c.ns, verrazzanoMonitoringInstance), &vmcontrollerv1.VerrazzanoMonitoringInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*vmcontrollerv1.VerrazzanoMonitoringInstance), err
}

// Delete takes name of the verrazzanoMonitoringInstance and deletes it. Returns an error if one occurs.
func (c *FakeVerrazzanoMonitoringInstances) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(verrazzanomonitoringinstancesResource, c.ns, name, opts), &vmcontrollerv1.VerrazzanoMonitoringInstance{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeVerrazzanoMonitoringInstances) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(verrazzanomonitoringinstancesResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &vmcontrollerv1.VerrazzanoMonitoringInstanceList{})
	return err
}

// Patch applies the patch and returns the patched verrazzanoMonitoringInstance.
func (c *FakeVerrazzanoMonitoringInstances) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *vmcontrollerv1.VerrazzanoMonitoringInstance, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(verrazzanomonitoringinstancesResource, c.ns, name, pt, data, subresources...), &vmcontrollerv1.VerrazzanoMonitoringInstance{})

	if obj == nil {
		return nil, err
	}
	return obj.(*vmcontrollerv1.VerrazzanoMonitoringInstance), err
}
