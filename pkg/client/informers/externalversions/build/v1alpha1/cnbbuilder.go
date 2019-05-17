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
	time "time"

	build_v1alpha1 "github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	versioned "github.com/pivotal/build-service-system/pkg/client/clientset/versioned"
	internalinterfaces "github.com/pivotal/build-service-system/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/pivotal/build-service-system/pkg/client/listers/build/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// CNBBuilderInformer provides access to a shared informer and lister for
// CNBBuilders.
type CNBBuilderInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.CNBBuilderLister
}

type cNBBuilderInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewCNBBuilderInformer constructs a new informer for CNBBuilder type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCNBBuilderInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCNBBuilderInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredCNBBuilderInformer constructs a new informer for CNBBuilder type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCNBBuilderInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.BuildV1alpha1().CNBBuilders(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.BuildV1alpha1().CNBBuilders(namespace).Watch(options)
			},
		},
		&build_v1alpha1.CNBBuilder{},
		resyncPeriod,
		indexers,
	)
}

func (f *cNBBuilderInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCNBBuilderInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *cNBBuilderInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&build_v1alpha1.CNBBuilder{}, f.defaultInformer)
}

func (f *cNBBuilderInformer) Lister() v1alpha1.CNBBuilderLister {
	return v1alpha1.NewCNBBuilderLister(f.Informer().GetIndexer())
}
