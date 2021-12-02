package v1alpha2

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func (i *SourceResolver) ConvertTo(_ context.Context, to apis.Convertible) error {
	switch toSourceResolver := to.(type) {
	case *v1alpha1.SourceResolver:
		toSourceResolver.ObjectMeta = i.ObjectMeta
		i.Spec.convertTo(&toSourceResolver.Spec)
		i.Status.convertTo(&toSourceResolver.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", toSourceResolver)
	}
	return nil
}

func (i *SourceResolver) ConvertFrom(_ context.Context, from apis.Convertible) error {
	switch fromSourceResolver := from.(type) {
	case *v1alpha1.SourceResolver:
		i.ObjectMeta = fromSourceResolver.ObjectMeta
		i.Spec.convertFrom(&fromSourceResolver.Spec)
		i.Status.convertFrom(&fromSourceResolver.Status)
	default:
		return fmt.Errorf("unknown version, got: %T", fromSourceResolver)
	}

	return nil
}

func (is *SourceResolverSpec) convertTo(to *v1alpha1.SourceResolverSpec) {
	to.Source = is.Source
	to.ServiceAccount = is.ServiceAccountName
}

func (is *SourceResolverSpec) convertFrom(from *v1alpha1.SourceResolverSpec) {
	is.Source = from.Source
	is.ServiceAccountName = from.ServiceAccount
}

func (is *SourceResolverStatus) convertFrom(from *v1alpha1.SourceResolverStatus) {
	is.Status = from.Status
	is.Source = from.Source
}

func (is *SourceResolverStatus) convertTo(to *v1alpha1.SourceResolverStatus) {
	to.Status = is.Status
	to.Source = is.Source
}
