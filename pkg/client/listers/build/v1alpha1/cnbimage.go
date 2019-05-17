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

// CNBImageLister helps list CNBImages.
type CNBImageLister interface {
	// List lists all CNBImages in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.CNBImage, err error)
	// CNBImages returns an object that can list and get CNBImages.
	CNBImages(namespace string) CNBImageNamespaceLister
	CNBImageListerExpansion
}

// cNBImageLister implements the CNBImageLister interface.
type cNBImageLister struct {
	indexer cache.Indexer
}

// NewCNBImageLister returns a new CNBImageLister.
func NewCNBImageLister(indexer cache.Indexer) CNBImageLister {
	return &cNBImageLister{indexer: indexer}
}

// List lists all CNBImages in the indexer.
func (s *cNBImageLister) List(selector labels.Selector) (ret []*v1alpha1.CNBImage, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CNBImage))
	})
	return ret, err
}

// CNBImages returns an object that can list and get CNBImages.
func (s *cNBImageLister) CNBImages(namespace string) CNBImageNamespaceLister {
	return cNBImageNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// CNBImageNamespaceLister helps list and get CNBImages.
type CNBImageNamespaceLister interface {
	// List lists all CNBImages in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.CNBImage, err error)
	// Get retrieves the CNBImage from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.CNBImage, error)
	CNBImageNamespaceListerExpansion
}

// cNBImageNamespaceLister implements the CNBImageNamespaceLister
// interface.
type cNBImageNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all CNBImages in the indexer for a given namespace.
func (s cNBImageNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.CNBImage, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.CNBImage))
	})
	return ret, err
}

// Get retrieves the CNBImage from the indexer for a given namespace and name.
func (s cNBImageNamespaceLister) Get(name string) (*v1alpha1.CNBImage, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("cnbimage"), name)
	}
	return obj.(*v1alpha1.CNBImage), nil
}
