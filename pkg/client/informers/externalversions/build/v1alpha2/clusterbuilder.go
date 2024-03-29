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

// Code generated by informer-gen. DO NOT EDIT.

package v1alpha2

import (
	"context"
	time "time"

	buildv1alpha2 "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	versioned "github.com/pivotal/kpack/pkg/client/clientset/versioned"
	internalinterfaces "github.com/pivotal/kpack/pkg/client/informers/externalversions/internalinterfaces"
	v1alpha2 "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// ClusterBuilderInformer provides access to a shared informer and lister for
// ClusterBuilders.
type ClusterBuilderInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha2.ClusterBuilderLister
}

type clusterBuilderInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewClusterBuilderInformer constructs a new informer for ClusterBuilder type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewClusterBuilderInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredClusterBuilderInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredClusterBuilderInformer constructs a new informer for ClusterBuilder type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredClusterBuilderInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KpackV1alpha2().ClusterBuilders().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.KpackV1alpha2().ClusterBuilders().Watch(context.TODO(), options)
			},
		},
		&buildv1alpha2.ClusterBuilder{},
		resyncPeriod,
		indexers,
	)
}

func (f *clusterBuilderInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredClusterBuilderInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *clusterBuilderInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&buildv1alpha2.ClusterBuilder{}, f.defaultInformer)
}

func (f *clusterBuilderInformer) Lister() v1alpha2.ClusterBuilderLister {
	return v1alpha2.NewClusterBuilderLister(f.Informer().GetIndexer())
}
