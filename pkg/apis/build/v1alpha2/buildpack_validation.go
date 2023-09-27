package v1alpha2

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

// TODO: add for extensions
func (cb *Buildpack) SetDefaults(context.Context) {
	if cb.Spec.ServiceAccountName == "" {
		cb.Spec.ServiceAccountName = "default"
	}
}

func (cb *Buildpack) Validate(ctx context.Context) *apis.FieldError {
	return cb.Spec.Validate(ctx).ViaField("spec")
}

func (s *BuildpackSpec) Validate(ctx context.Context) *apis.FieldError {
	return validate.Image(s.Image)
}
