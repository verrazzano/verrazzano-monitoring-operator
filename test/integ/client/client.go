// Copyright (C) 2020, Oracle and/or its affiliates.
// Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl.
package client

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"

	vmcontrollerv1 "github.com/verrazzano/verrazzano-monitoring-operator/pkg/apis/vmcontroller/v1"
)

// VMICR provides a client interface for vmi CRs.
type VMICR interface {
	// Create creates a vmi CR with the desired CR.
	Create(ctx context.Context, cl *vmcontrollerv1.VerrazzanoMonitoringInstance) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error)

	// Get returns the specified vmi CR.
	Get(ctx context.Context, namespace, name string) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error)

	// Delete deletes the specified vmi CR.
	Delete(ctx context.Context, namespace, name string) error

	// Update updates the vmi CR.
	Update(ctx context.Context, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error)
}

type vmiCR struct {
	client     *rest.RESTClient
	crScheme   *runtime.Scheme
	paramCodec runtime.ParameterCodec
}

// NewCRClient creates a new vmi CR client.
func NewCRClient(cfg *rest.Config) (VMICR, error) {
	cli, crScheme, err := new1(cfg)
	if err != nil {
		return nil, err
	}
	return &vmiCR{
		client:     cli,
		crScheme:   crScheme,
		paramCodec: runtime.NewParameterCodec(crScheme),
	}, nil
}

func new1(cfg *rest.Config) (*rest.RESTClient, *runtime.Scheme, error) {
	crScheme := runtime.NewScheme()
	if err := vmcontrollerv1.AddToScheme(crScheme); err != nil {
		return nil, nil, err
	}

	config := *cfg
	config.GroupVersion = &vmcontrollerv1.SchemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{
		CodecFactory: serializer.NewCodecFactory(crScheme),
	}

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, nil, err
	}

	return client, crScheme, nil
}

func (c *vmiCR) Create(ctx context.Context, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error) {
	if len(vmi.Namespace) == 0 {
		return nil, errors.New("need to set metadata.Namespace in vmi CR")
	}
	result := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	err := c.client.Post().
		Namespace(vmi.Namespace).
		Resource("verrazzanomonitoringinstances").
		Body(vmi).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *vmiCR) Get(ctx context.Context, namespace, name string) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error) {
	result := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	err := c.client.Get().
		Namespace(namespace).
		Resource("verrazzanomonitoringinstances").
		Name(name).
		Do(ctx).
		Into(result)
	return result, err
}

func (c *vmiCR) Delete(ctx context.Context, namespace, name string) error {
	return c.client.Delete().
		Namespace(namespace).
		Resource("verrazzanomonitoringinstances").
		Name(name).
		Do(ctx).
		Error()
}

func (c *vmiCR) Update(ctx context.Context, vmi *vmcontrollerv1.VerrazzanoMonitoringInstance) (*vmcontrollerv1.VerrazzanoMonitoringInstance, error) {
	if len(vmi.Namespace) == 0 {
		return nil, errors.New("need to set metadata.Namespace in vmi CR")
	}
	if len(vmi.Name) == 0 {
		return nil, errors.New("need to set metadata.Name in vmi CR")
	}

	result := &vmcontrollerv1.VerrazzanoMonitoringInstance{}
	err := c.client.Put().
		Namespace(vmi.Namespace).
		Resource("verrazzanomonitoringinstances").
		Name(vmi.Name).
		Body(vmi).
		Do(ctx).
		Into(result)
	return result, err
}
