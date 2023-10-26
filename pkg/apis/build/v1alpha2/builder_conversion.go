package v1alpha2

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func (b *Builder) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toBuilder := to.(type) {
	case *v1alpha1.Builder:
		toBuilder.ObjectMeta = b.ObjectMeta

		b.Spec.convertTo(&toBuilder.Spec)
		b.Status.convertTo(&toBuilder.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", toBuilder)
	}
	return nil
}

func (b *Builder) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromBuilder := from.(type) {
	case *v1alpha1.Builder:
		b.ObjectMeta = fromBuilder.ObjectMeta
		b.Spec.convertFrom(&fromBuilder.Spec)
		b.Status.convertFrom(&fromBuilder.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", fromBuilder)
	}

	return nil
}

func (bs *NamespacedBuilderSpec) convertTo(to *v1alpha1.NamespacedBuilderSpec) {
	to.Tag = bs.Tag
	to.Stack = bs.Stack
	to.Store = bs.Store
	to.ServiceAccount = bs.ServiceAccount()

	for _, builderOrderEntry := range bs.Order {
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

func (bs *NamespacedBuilderSpec) convertFrom(from *v1alpha1.NamespacedBuilderSpec) {
	bs.Tag = from.Tag
	bs.Stack = from.Stack
	bs.Store = from.Store
	bs.ServiceAccountName = from.ServiceAccount

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
		bs.Order = append(bs.Order, builderOrderEntry)
	}
}

func (bst *BuilderStatus) convertFrom(from *v1alpha1.BuilderStatus) {
	bst.Status = from.Status
	bst.BuilderMetadataBuildpacks = from.BuilderMetadata
	bst.Order = from.Order
	bst.Stack = from.Stack
	bst.LatestImage = from.LatestImage
	bst.ObservedStackGeneration = from.ObservedStackGeneration
	bst.ObservedStoreGeneration = from.ObservedStoreGeneration
	bst.OS = from.OS
}

func (bst *BuilderStatus) convertTo(to *v1alpha1.BuilderStatus) {
	to.Status = bst.Status
	to.BuilderMetadata = bst.BuilderMetadataBuildpacks
	to.Order = bst.Order
	to.Stack = bst.Stack
	to.LatestImage = bst.LatestImage
	to.ObservedStackGeneration = bst.ObservedStackGeneration
	to.ObservedStoreGeneration = bst.ObservedStoreGeneration
	to.OS = bst.OS
}
