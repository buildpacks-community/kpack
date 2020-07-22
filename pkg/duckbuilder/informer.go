package duckbuilder

import (
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	informerv1alpha1 "github.com/pivotal/kpack/pkg/client/informers/externalversions/build/v1alpha1"
	v1alpha1Listers "github.com/pivotal/kpack/pkg/client/listers/build/v1alpha1"
)

type DuckBuilderInformer struct {
	CustomBuilderInformer        informerv1alpha1.CustomBuilderInformer
	CustomClusterBuilderInformer informerv1alpha1.CustomClusterBuilderInformer
}

func (di *DuckBuilderInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	di.CustomBuilderInformer.Informer().AddEventHandler(handler)
	di.CustomClusterBuilderInformer.Informer().AddEventHandler(handler)
}

func (di *DuckBuilderInformer) Lister() *DuckBuilderLister {
	return &DuckBuilderLister{
		CustomBuilderLister:        di.CustomBuilderInformer.Lister(),
		CustomClusterBuilderLister: di.CustomClusterBuilderInformer.Lister(),
	}
}

type DuckBuilderLister struct {
	CustomBuilderLister        v1alpha1Listers.CustomBuilderLister
	CustomClusterBuilderLister v1alpha1Listers.CustomClusterBuilderLister
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
	case v1alpha1.CustomBuilderKind:
		builder, err := bl.DuckBuilderLister.CustomBuilderLister.CustomBuilders(bl.namespace).Get(reference.Name)
		return convertCustomBuilder(builder), err
	case v1alpha1.CustomClusterBuilderKind:
		builder, err := bl.DuckBuilderLister.CustomClusterBuilderLister.Get(reference.Name)
		return convertCustomClusterBuilder(builder), err
	default:
		return nil, errors.Errorf("unknown builder type: %s", reference.Kind)
	}
}

func convertCustomBuilder(builder *v1alpha1.CustomBuilder) *DuckBuilder {
	if builder == nil {
		return nil
	}

	return &DuckBuilder{
		TypeMeta:   builder.TypeMeta,
		ObjectMeta: builder.ObjectMeta,
		Status:     builder.Status,
	}
}

func convertCustomClusterBuilder(builder *v1alpha1.CustomClusterBuilder) *DuckBuilder {
	if builder == nil {
		return nil
	}

	return &DuckBuilder{
		TypeMeta:   builder.TypeMeta,
		ObjectMeta: builder.ObjectMeta,
		Status:     builder.Status,
	}
}
