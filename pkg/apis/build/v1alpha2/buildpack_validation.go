package v1alpha2

import (
	"context"

	"github.com/pivotal/kpack/pkg/apis/validate"
	"knative.dev/pkg/apis"
)

func (cb *Buildpack) SetDefaults(context.Context) {
	if cb.Spec.ServiceAccountName == "" {
		cb.Spec.ServiceAccountName = "default"
	}
}

func (cb *Buildpack) Validate(ctx context.Context) *apis.FieldError {
	return cb.Spec.Validate(ctx).ViaField("spec")
}

func (s *BuildpackSpec) Validate(ctx context.Context) *apis.FieldError {
	return validate.Image(s.Source.Image).ViaField("source")
}
