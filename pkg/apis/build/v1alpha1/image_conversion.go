package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
)

func (i *Image) ConvertTo(context.Context, apis.Convertible) error {
	return nil
}

func (i *Image) ConvertFrom(context.Context, apis.Convertible) error {
	return nil
}
