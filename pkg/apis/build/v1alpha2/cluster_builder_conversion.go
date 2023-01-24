package v1alpha2

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
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
	to.ServiceAccountRef = cs.ServiceAccountRef

	for _, builderOrderEntry := range cs.Order {
		var coreOrderEntry corev1alpha1.OrderEntry
		for _, ref := range builderOrderEntry.Group {
			if ref.Id != "" {
				coreOrderEntry.Group = append(coreOrderEntry.Group,
					corev1alpha1.BuildpackRef{
						BuildpackInfo: corev1alpha1.BuildpackInfo{Id: ref.Id, Version: ref.Version},
						Optional:      ref.Optional,
					},
				)
			}
		}
		to.Order = append(to.Order, coreOrderEntry)
	}
}

func (cs *ClusterBuilderSpec) convertFrom(from *v1alpha1.ClusterBuilderSpec) {
	cs.Tag = from.Tag
	cs.Stack = from.Stack
	cs.Store = from.Store
	cs.ServiceAccountRef = from.ServiceAccountRef

	for _, coreOrderEntry := range from.Order {
		var builderOrderEntry BuilderOrderEntry
		for _, ref := range coreOrderEntry.Group {
			builderOrderEntry.Group = append(builderOrderEntry.Group,
				BuilderBuildpackRef{
					BuildpackRef: corev1alpha1.BuildpackRef{
						BuildpackInfo: corev1alpha1.BuildpackInfo{Id: ref.Id, Version: ref.Version},
						Optional:      ref.Optional,
					},
				},
			)
		}
		cs.Order = append(cs.Order, builderOrderEntry)
	}
}
