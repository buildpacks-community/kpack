/*
 * Copyright 2019 The original author or authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package v1alpha1

import (
	v1alpha1 "github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	scheme "github.com/pivotal/build-service-system/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// CNBBuildersGetter has a method to return a CNBBuilderInterface.
// A group's client should implement this interface.
type CNBBuildersGetter interface {
	CNBBuilders(namespace string) CNBBuilderInterface
}

// CNBBuilderInterface has methods to work with CNBBuilder resources.
type CNBBuilderInterface interface {
	Create(*v1alpha1.CNBBuilder) (*v1alpha1.CNBBuilder, error)
	Update(*v1alpha1.CNBBuilder) (*v1alpha1.CNBBuilder, error)
	UpdateStatus(*v1alpha1.CNBBuilder) (*v1alpha1.CNBBuilder, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.CNBBuilder, error)
	List(opts v1.ListOptions) (*v1alpha1.CNBBuilderList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.CNBBuilder, err error)
	CNBBuilderExpansion
}

// cNBBuilders implements CNBBuilderInterface
type cNBBuilders struct {
	client rest.Interface
	ns     string
}

// newCNBBuilders returns a CNBBuilders
func newCNBBuilders(c *BuildV1alpha1Client, namespace string) *cNBBuilders {
	return &cNBBuilders{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the cNBBuilder, and returns the corresponding cNBBuilder object, and an error if there is any.
func (c *cNBBuilders) Get(name string, options v1.GetOptions) (result *v1alpha1.CNBBuilder, err error) {
	result = &v1alpha1.CNBBuilder{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("cnbbuilders").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of CNBBuilders that match those selectors.
func (c *cNBBuilders) List(opts v1.ListOptions) (result *v1alpha1.CNBBuilderList, err error) {
	result = &v1alpha1.CNBBuilderList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("cnbbuilders").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested cNBBuilders.
func (c *cNBBuilders) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("cnbbuilders").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a cNBBuilder and creates it.  Returns the server's representation of the cNBBuilder, and an error, if there is any.
func (c *cNBBuilders) Create(cNBBuilder *v1alpha1.CNBBuilder) (result *v1alpha1.CNBBuilder, err error) {
	result = &v1alpha1.CNBBuilder{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("cnbbuilders").
		Body(cNBBuilder).
		Do().
		Into(result)
	return
}

// Update takes the representation of a cNBBuilder and updates it. Returns the server's representation of the cNBBuilder, and an error, if there is any.
func (c *cNBBuilders) Update(cNBBuilder *v1alpha1.CNBBuilder) (result *v1alpha1.CNBBuilder, err error) {
	result = &v1alpha1.CNBBuilder{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("cnbbuilders").
		Name(cNBBuilder.Name).
		Body(cNBBuilder).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *cNBBuilders) UpdateStatus(cNBBuilder *v1alpha1.CNBBuilder) (result *v1alpha1.CNBBuilder, err error) {
	result = &v1alpha1.CNBBuilder{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("cnbbuilders").
		Name(cNBBuilder.Name).
		SubResource("status").
		Body(cNBBuilder).
		Do().
		Into(result)
	return
}

// Delete takes name of the cNBBuilder and deletes it. Returns an error if one occurs.
func (c *cNBBuilders) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("cnbbuilders").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *cNBBuilders) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("cnbbuilders").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched cNBBuilder.
func (c *cNBBuilders) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.CNBBuilder, err error) {
	result = &v1alpha1.CNBBuilder{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("cnbbuilders").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
