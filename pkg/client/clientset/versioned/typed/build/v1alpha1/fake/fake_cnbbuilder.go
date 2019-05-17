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
package fake

import (
	v1alpha1 "github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeCNBBuilders implements CNBBuilderInterface
type FakeCNBBuilders struct {
	Fake *FakeBuildV1alpha1
	ns   string
}

var cnbbuildersResource = schema.GroupVersionResource{Group: "build.pivotal.io", Version: "v1alpha1", Resource: "cnbbuilders"}

var cnbbuildersKind = schema.GroupVersionKind{Group: "build.pivotal.io", Version: "v1alpha1", Kind: "CNBBuilder"}

// Get takes name of the cNBBuilder, and returns the corresponding cNBBuilder object, and an error if there is any.
func (c *FakeCNBBuilders) Get(name string, options v1.GetOptions) (result *v1alpha1.CNBBuilder, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(cnbbuildersResource, c.ns, name), &v1alpha1.CNBBuilder{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CNBBuilder), err
}

// List takes label and field selectors, and returns the list of CNBBuilders that match those selectors.
func (c *FakeCNBBuilders) List(opts v1.ListOptions) (result *v1alpha1.CNBBuilderList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(cnbbuildersResource, cnbbuildersKind, c.ns, opts), &v1alpha1.CNBBuilderList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.CNBBuilderList{ListMeta: obj.(*v1alpha1.CNBBuilderList).ListMeta}
	for _, item := range obj.(*v1alpha1.CNBBuilderList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested cNBBuilders.
func (c *FakeCNBBuilders) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(cnbbuildersResource, c.ns, opts))

}

// Create takes the representation of a cNBBuilder and creates it.  Returns the server's representation of the cNBBuilder, and an error, if there is any.
func (c *FakeCNBBuilders) Create(cNBBuilder *v1alpha1.CNBBuilder) (result *v1alpha1.CNBBuilder, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(cnbbuildersResource, c.ns, cNBBuilder), &v1alpha1.CNBBuilder{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CNBBuilder), err
}

// Update takes the representation of a cNBBuilder and updates it. Returns the server's representation of the cNBBuilder, and an error, if there is any.
func (c *FakeCNBBuilders) Update(cNBBuilder *v1alpha1.CNBBuilder) (result *v1alpha1.CNBBuilder, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(cnbbuildersResource, c.ns, cNBBuilder), &v1alpha1.CNBBuilder{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CNBBuilder), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeCNBBuilders) UpdateStatus(cNBBuilder *v1alpha1.CNBBuilder) (*v1alpha1.CNBBuilder, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(cnbbuildersResource, "status", c.ns, cNBBuilder), &v1alpha1.CNBBuilder{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CNBBuilder), err
}

// Delete takes name of the cNBBuilder and deletes it. Returns an error if one occurs.
func (c *FakeCNBBuilders) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(cnbbuildersResource, c.ns, name), &v1alpha1.CNBBuilder{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeCNBBuilders) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(cnbbuildersResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.CNBBuilderList{})
	return err
}

// Patch applies the patch and returns the patched cNBBuilder.
func (c *FakeCNBBuilders) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.CNBBuilder, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(cnbbuildersResource, c.ns, name, data, subresources...), &v1alpha1.CNBBuilder{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.CNBBuilder), err
}
