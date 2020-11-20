package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
)

func (b *Build) ConvertTo(context.Context, apis.Convertible) error {
	return nil
}

func (b *Build) ConvertFrom(context.Context, apis.Convertible) error {
	return nil
}
