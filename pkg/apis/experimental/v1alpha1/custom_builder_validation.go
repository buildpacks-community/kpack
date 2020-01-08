package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

const (
	defaultServiceAccount = "default"
)

func (cb *CustomBuilder) SetDefaults(context.Context) {
	if cb.Spec.ServiceAccount == "" {
		cb.Spec.ServiceAccount = defaultServiceAccount
	}
}

func (cb *CustomBuilder) Validate(ctx context.Context) *apis.FieldError {
	return cb.Spec.Validate(ctx).ViaField("spec")
}

func (s *CustomBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return validate.Tag(s.Tag).
		Also(validate.FieldNotEmpty(s.Stack, "stack")).
		Also(validate.FieldNotEmpty(s.Store, "store"))
}

func (s *CustomNamespacedBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return s.CustomBuilderSpec.Validate(ctx).
		Also(validate.FieldNotEmpty(s.ServiceAccount, "serviceAccount"))
}
