package v1alpha2

import (
	"context"
	"fmt"
	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (i *Builder) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toBuilder := to.(type) {
	case *v1alpha1.Builder:
		toBuilder.ObjectMeta = i.ObjectMeta
		i.Spec.convertTo(&toBuilder.Spec)
		i.Status.convertTo(&toBuilder.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", toBuilder)
	}
	return nil
}

func (i *Builder) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromBuilder := from.(type) {
	case *v1alpha1.Builder:
		i.ObjectMeta = fromBuilder.ObjectMeta
		i.Spec.convertFrom(&fromBuilder.Spec)
		i.Status.convertFrom(&fromBuilder.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", fromBuilder)
	}

	return nil
}

func (is *NamespacedBuilderSpec) convertTo(to *v1alpha1.NamespacedBuilderSpec) {
	to.Tag = is.Tag
	to.Stack = is.Stack
	to.Store = is.Store
	to.Order = is.Order
	to.ServiceAccount = is.ServiceAccount()
}

func (is *NamespacedBuilderSpec) convertFrom(from *v1alpha1.NamespacedBuilderSpec) {
	is.Tag = from.Tag
	is.Stack = from.Stack
	is.Store = from.Store
	is.Order = from.Order
	is.ServiceAccountName = from.ServiceAccount
}

func (is *BuilderStatus) convertFrom(from *v1alpha1.BuilderStatus) {
	is.Status = from.Status
	is.BuilderMetadata = from.BuilderMetadata
	is.Order = from.Order
	is.Stack = from.Stack
	is.LatestImage = from.LatestImage
	is.ObservedStackGeneration = from.ObservedStackGeneration
	is.ObservedStoreGeneration = from.ObservedStoreGeneration
	is.OS = from.OS
}

func (is *BuilderStatus) convertTo(to *v1alpha1.BuilderStatus) {
	to.Status = is.Status
	to.BuilderMetadata = is.BuilderMetadata
	to.Order = is.Order
	to.Stack = is.Stack
	to.LatestImage = is.LatestImage
	to.ObservedStackGeneration = is.ObservedStackGeneration
	to.ObservedStoreGeneration = is.ObservedStoreGeneration
	to.OS = is.OS
}
