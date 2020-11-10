package v1alpha1

import (
	"context"
	"knative.dev/pkg/apis"
)

func (b *Build) ConvertTo(ctx context.Context, to apis.Convertible) error {
	return nil
}

func (b *Build) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	return nil
}
