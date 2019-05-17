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

// CNBImagesGetter has a method to return a CNBImageInterface.
// A group's client should implement this interface.
type CNBImagesGetter interface {
	CNBImages(namespace string) CNBImageInterface
}

// CNBImageInterface has methods to work with CNBImage resources.
type CNBImageInterface interface {
	Create(*v1alpha1.CNBImage) (*v1alpha1.CNBImage, error)
	Update(*v1alpha1.CNBImage) (*v1alpha1.CNBImage, error)
	UpdateStatus(*v1alpha1.CNBImage) (*v1alpha1.CNBImage, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.CNBImage, error)
	List(opts v1.ListOptions) (*v1alpha1.CNBImageList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.CNBImage, err error)
	CNBImageExpansion
}

// cNBImages implements CNBImageInterface
type cNBImages struct {
	client rest.Interface
	ns     string
}

// newCNBImages returns a CNBImages
func newCNBImages(c *BuildV1alpha1Client, namespace string) *cNBImages {
	return &cNBImages{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the cNBImage, and returns the corresponding cNBImage object, and an error if there is any.
func (c *cNBImages) Get(name string, options v1.GetOptions) (result *v1alpha1.CNBImage, err error) {
	result = &v1alpha1.CNBImage{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("cnbimages").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of CNBImages that match those selectors.
func (c *cNBImages) List(opts v1.ListOptions) (result *v1alpha1.CNBImageList, err error) {
	result = &v1alpha1.CNBImageList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("cnbimages").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested cNBImages.
func (c *cNBImages) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("cnbimages").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a cNBImage and creates it.  Returns the server's representation of the cNBImage, and an error, if there is any.
func (c *cNBImages) Create(cNBImage *v1alpha1.CNBImage) (result *v1alpha1.CNBImage, err error) {
	result = &v1alpha1.CNBImage{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("cnbimages").
		Body(cNBImage).
		Do().
		Into(result)
	return
}

// Update takes the representation of a cNBImage and updates it. Returns the server's representation of the cNBImage, and an error, if there is any.
func (c *cNBImages) Update(cNBImage *v1alpha1.CNBImage) (result *v1alpha1.CNBImage, err error) {
	result = &v1alpha1.CNBImage{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("cnbimages").
		Name(cNBImage.Name).
		Body(cNBImage).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *cNBImages) UpdateStatus(cNBImage *v1alpha1.CNBImage) (result *v1alpha1.CNBImage, err error) {
	result = &v1alpha1.CNBImage{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("cnbimages").
		Name(cNBImage.Name).
		SubResource("status").
		Body(cNBImage).
		Do().
		Into(result)
	return
}

// Delete takes name of the cNBImage and deletes it. Returns an error if one occurs.
func (c *cNBImages) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("cnbimages").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *cNBImages) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("cnbimages").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched cNBImage.
func (c *cNBImages) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.CNBImage, err error) {
	result = &v1alpha1.CNBImage{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("cnbimages").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
