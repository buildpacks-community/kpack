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

// CNBImageInformer provides access to a shared informer and lister for
// CNBImages.
type CNBImageInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.CNBImageLister
}

type cNBImageInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewCNBImageInformer constructs a new informer for CNBImage type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewCNBImageInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredCNBImageInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredCNBImageInformer constructs a new informer for CNBImage type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredCNBImageInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.BuildV1alpha1().CNBImages(namespace).List(options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.BuildV1alpha1().CNBImages(namespace).Watch(options)
			},
		},
		&build_v1alpha1.CNBImage{},
		resyncPeriod,
		indexers,
	)
}

func (f *cNBImageInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredCNBImageInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *cNBImageInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&build_v1alpha1.CNBImage{}, f.defaultInformer)
}

func (f *cNBImageInformer) Lister() v1alpha1.CNBImageLister {
	return v1alpha1.NewCNBImageLister(f.Informer().GetIndexer())
}
