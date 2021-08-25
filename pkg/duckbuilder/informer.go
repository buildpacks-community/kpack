package duckbuilder

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	buildinformers "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha2"
	buildlisters "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha2"
)

type DuckBuilderInformer struct {
	BuilderInformer        buildinformers.BuilderInformer
	ClusterBuilderInformer buildinformers.ClusterBuilderInformer
}

func (di *DuckBuilderInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	di.BuilderInformer.Informer().AddEventHandler(handler)
	di.ClusterBuilderInformer.Informer().AddEventHandler(handler)
}

func (di *DuckBuilderInformer) Lister() *DuckBuilderLister {
	return &DuckBuilderLister{
		BuilderLister:        di.BuilderInformer.Lister(),
		ClusterBuilderLister: di.ClusterBuilderInformer.Lister(),
	}
}

type DuckBuilderLister struct {
	BuilderLister        buildlisters.BuilderLister
	ClusterBuilderLister buildlisters.ClusterBuilderLister
}

func (bl *DuckBuilderLister) Namespace(namespace string) *DuckBuilderNamespaceLister {
	return &DuckBuilderNamespaceLister{
		DuckBuilderLister: bl,
		namespace:         namespace,
	}
}

type DuckBuilderNamespaceLister struct {
	DuckBuilderLister *DuckBuilderLister
	namespace         string
}

func (bl *DuckBuilderNamespaceLister) Get(reference corev1.ObjectReference) (*DuckBuilder, error) {
	switch reference.Kind {
	case buildapi.BuilderKind:
		builder, err := bl.DuckBuilderLister.BuilderLister.Builders(bl.namespace).Get(reference.Name)
		return convertBuilder(builder), err
	case buildapi.ClusterBuilderKind:
		builder, err := bl.DuckBuilderLister.ClusterBuilderLister.Get(reference.Name)
		return convertClusterBuilder(builder), err
	default:
		return nil, errors.Errorf("unknown builder type: %s", reference.Kind)
	}
}

func convertBuilder(builder *buildapi.Builder) *DuckBuilder {
	if builder == nil {
		return nil
	}

	return &DuckBuilder{
		TypeMeta:   builder.TypeMeta,
		ObjectMeta: builder.ObjectMeta,
		Status:     builder.Status,
	}
}

func convertClusterBuilder(builder *buildapi.ClusterBuilder) *DuckBuilder {
	if builder == nil {
		return nil
	}

	return &DuckBuilder{
		TypeMeta:   builder.TypeMeta,
		ObjectMeta: builder.ObjectMeta,
		Status:     builder.Status,
	}
}
