package v1alpha2

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

func (e *Extension) SetDefaults(context.Context) {
	if e.Spec.ServiceAccountName == "" {
		e.Spec.ServiceAccountName = "default"
	}
}

func (e *Extension) Validate(ctx context.Context) *apis.FieldError {
	return e.Spec.Validate(ctx).ViaField("spec")
}

func (s *ExtensionSpec) Validate(ctx context.Context) *apis.FieldError {
	return validate.Image(s.Image)
}
