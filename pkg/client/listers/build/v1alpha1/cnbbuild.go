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

// CNBBuildLister helps list CNBBuilds.
type CNBBuildLister interface {
	// List lists all CNBBuilds in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.CNBBuild, err error)
	// CNBBuilds returns an object that can list and get CNBBuilds.
	CNBBuilds(namespace string) CNBBuildNamespaceLister
	CNBBuildListerExpansion
}

// cNBBuildLister implements the CNBBuildLister interface.
type cNBBuildLister struct {
	indexer cache.Indexer
}

// NewCNBBuildLister returns a new CNBBuildLister.
func NewCNBBuildLister(indexer cache.Indexer) CNBBuildLister {
	return &cNBBuildLister{indexer: indexer}
}

// List lists all CNBBuilds in the indexer.
func (s *cNBBuildLister) List(selector labels.Selector) (ret []*v1alpha1.CNBBuild, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CNBBuild))
	})
	return ret, err
}

// CNBBuilds returns an object that can list and get CNBBuilds.
func (s *cNBBuildLister) CNBBuilds(namespace string) CNBBuildNamespaceLister {
	return cNBBuildNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// CNBBuildNamespaceLister helps list and get CNBBuilds.
type CNBBuildNamespaceLister interface {
	// List lists all CNBBuilds in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.CNBBuild, err error)
	// Get retrieves the CNBBuild from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.CNBBuild, error)
	CNBBuildNamespaceListerExpansion
}

// cNBBuildNamespaceLister implements the CNBBuildNamespaceLister
// interface.
type cNBBuildNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all CNBBuilds in the indexer for a given namespace.
func (s cNBBuildNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.CNBBuild, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CNBBuild))
	})
	return ret, err
}

// Get retrieves the CNBBuild from the indexer for a given namespace and name.
func (s cNBBuildNamespaceLister) Get(name string) (*v1alpha1.CNBBuild, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("cnbbuild"), name)
	}
	return obj.(*v1alpha1.CNBBuild), nil
}
