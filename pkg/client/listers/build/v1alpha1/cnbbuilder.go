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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// CNBBuilderLister helps list CNBBuilders.
type CNBBuilderLister interface {
	// List lists all CNBBuilders in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.CNBBuilder, err error)
	// CNBBuilders returns an object that can list and get CNBBuilders.
	CNBBuilders(namespace string) CNBBuilderNamespaceLister
	CNBBuilderListerExpansion
}

// cNBBuilderLister implements the CNBBuilderLister interface.
type cNBBuilderLister struct {
	indexer cache.Indexer
}

// NewCNBBuilderLister returns a new CNBBuilderLister.
func NewCNBBuilderLister(indexer cache.Indexer) CNBBuilderLister {
	return &cNBBuilderLister{indexer: indexer}
}

// List lists all CNBBuilders in the indexer.
func (s *cNBBuilderLister) List(selector labels.Selector) (ret []*v1alpha1.CNBBuilder, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CNBBuilder))
	})
	return ret, err
}

// CNBBuilders returns an object that can list and get CNBBuilders.
func (s *cNBBuilderLister) CNBBuilders(namespace string) CNBBuilderNamespaceLister {
	return cNBBuilderNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// CNBBuilderNamespaceLister helps list and get CNBBuilders.
type CNBBuilderNamespaceLister interface {
	// List lists all CNBBuilders in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.CNBBuilder, err error)
	// Get retrieves the CNBBuilder from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.CNBBuilder, error)
	CNBBuilderNamespaceListerExpansion
}

// cNBBuilderNamespaceLister implements the CNBBuilderNamespaceLister
// interface.
type cNBBuilderNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all CNBBuilders in the indexer for a given namespace.
func (s cNBBuilderNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.CNBBuilder, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CNBBuilder))
	})
	return ret, err
}

// Get retrieves the CNBBuilder from the indexer for a given namespace and name.
func (s cNBBuilderNamespaceLister) Get(name string) (*v1alpha1.CNBBuilder, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("cnbbuilder"), name)
	}
	return obj.(*v1alpha1.CNBBuilder), nil
}
