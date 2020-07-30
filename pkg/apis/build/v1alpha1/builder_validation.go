package v1alpha1

import (
	"context"

	v1 "k8s.io/api/core/v1"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

func (cb *Builder) SetDefaults(context.Context) {
	if cb.Spec.ServiceAccount == "" {
		cb.Spec.ServiceAccount = "default"
	}
	if cb.Spec.Stack.Kind == "" {
		cb.Spec.Stack.Kind = ClusterStackKind
	}
	if cb.Spec.Store.Kind == "" {
		cb.Spec.Store.Kind = ClusterStoreKind
	}
}

func (cb *Builder) Validate(ctx context.Context) *apis.FieldError {
	return cb.Spec.Validate(ctx).ViaField("spec")
}

func (s *BuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return validate.Tag(s.Tag).
		Also(validateStack(s.Stack).ViaField("stack")).
		Also(validateStore(s.Store).ViaField("store"))
}

func (s *NamespacedBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return s.BuilderSpec.Validate(ctx).
		Also(validate.FieldNotEmpty(s.ServiceAccount, "serviceAccount"))
}

func validateStack(stack v1.ObjectReference) *apis.FieldError {
	if stack.Name == "" {
		return apis.ErrMissingField("name")
	}

	switch stack.Kind {
	case ClusterStackKind:
		return nil
	default:
		return apis.ErrInvalidValue(stack.Kind, "kind")
	}
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
