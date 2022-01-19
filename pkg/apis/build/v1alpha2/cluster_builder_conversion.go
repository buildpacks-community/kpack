package v1alpha2

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (b *ClusterBuilder) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toClusterBuilder := to.(type) {
	case *v1alpha1.ClusterBuilder:
		toClusterBuilder.ObjectMeta = b.ObjectMeta

		b.Spec.convertTo(&toClusterBuilder.Spec)
		b.Status.convertTo(&toClusterBuilder.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", toClusterBuilder)
	}
	return nil
}

func (b *ClusterBuilder) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromClusterBuilder := from.(type) {
	case *v1alpha1.ClusterBuilder:
		b.ObjectMeta = fromClusterBuilder.ObjectMeta
		b.Spec.convertFrom(&fromClusterBuilder.Spec)
		b.Status.convertFrom(&fromClusterBuilder.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", fromClusterBuilder)
	}

	return nil
}

func (cs *ClusterBuilderSpec) convertTo(to *v1alpha1.ClusterBuilderSpec) {
	to.Tag = cs.Tag
	to.Stack = cs.Stack
	to.Store = cs.Store
	to.Order = cs.Order
	to.ServiceAccountRef = cs.ServiceAccountRef
}

func (cs *ClusterBuilderSpec) convertFrom(from *v1alpha1.ClusterBuilderSpec) {
	cs.Tag = from.Tag
	cs.Stack = from.Stack
	cs.Store = from.Store
	cs.Order = from.Order
	cs.ServiceAccountRef = from.ServiceAccountRef
}
