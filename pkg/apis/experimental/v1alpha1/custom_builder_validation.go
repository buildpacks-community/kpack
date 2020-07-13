package v1alpha1

import (
	"context"
	v1 "k8s.io/api/core/v1"

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
		Also(validateStore(s.Store).ViaField("store"))
}

func (s *CustomNamespacedBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return s.CustomBuilderSpec.Validate(ctx).
		Also(validate.FieldNotEmpty(s.ServiceAccount, "serviceAccount"))
}

func validateStore(store v1.ObjectReference) *apis.FieldError {
	if store.Name == "" {
		return apis.ErrMissingField("name")
	}

	switch store.Kind {
	case ClusterStoreKind:
		return nil
	default:
		return apis.ErrInvalidValue(store.Kind, "kind")
	}
}
