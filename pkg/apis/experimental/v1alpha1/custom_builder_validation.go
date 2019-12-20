package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
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
	return v1alpha1.ValidateTag(s.Tag).
		Also(s.Stack.Validate(ctx).ViaField("stack")).
		Also(v1alpha1.ValidateFieldNotEmpty(s.Store, "store"))
}

func (s *CustomNamespacedBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return s.CustomBuilderSpec.Validate(ctx).
		Also(v1alpha1.ValidateFieldNotEmpty(s.ServiceAccount, "serviceAccount"))
}

func (s *Stack) Validate(ctx context.Context) *apis.FieldError {
	return v1alpha1.ValidateImage(s.BaseBuilderImage)
}
