package v1alpha2

import (
	"context"

	v1 "k8s.io/api/core/v1"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

func (cb *Builder) SetDefaults(context.Context) {
	if cb.Spec.ServiceAccount() == "" {
		cb.Spec.ServiceAccountName = "default"
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
		Also(validateOrder(s.Order).ViaField("order"))
}

func (s *NamespacedBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return s.BuilderSpec.Validate(ctx).
		Also(validate.FieldNotEmpty(s.ServiceAccount(), "serviceAccountName"))
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

func validateOrder(order []BuilderOrderEntry) *apis.FieldError {
	var errs *apis.FieldError
	for i, s := range order {
		errs.Also(validateGroup(s).ViaIndex(i))
	}
	return errs
}

func validateGroup(group BuilderOrderEntry) *apis.FieldError {
	var errs *apis.FieldError
	for i, s := range group.Group {
		errs.Also(validateBuildpackRef(s).ViaIndex(i).ViaField("group"))
	}
	return errs
}

func validateBuildpackRef(ref BuilderBuildpackRef) *apis.FieldError {
	return nil
}
