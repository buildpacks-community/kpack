package v1alpha1

import (
	"context"
	"knative.dev/pkg/apis"
)

func (i *Image) ConvertTo(ctx context.Context, to apis.Convertible) error {
	return nil
}

func (i *Image) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	return nil
}
