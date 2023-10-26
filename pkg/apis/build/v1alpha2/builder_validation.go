package v1alpha2

import (
	"context"
	"strings"

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
		Also(validateStore(s.Store).ViaField("store")).
		Also(validateOrder(s.Order).ViaField("order")).
		Also(validateOrderExtensions(s.OrderExtensions).ViaField("order-extensions"))
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
	if store.Name == "" && store.Kind == "" {
		return nil
	}
	return validateObjectRef(store, []string{ClusterStoreKind})
}

func validateOrder(order []BuilderOrderEntry) *apis.FieldError {
	var errs *apis.FieldError
	for i, s := range order {
		errs = errs.Also(validateGroup(s).ViaIndex(i))
	}
	return errs
}

func validateOrderExtensions(orderExt []BuilderOrderEntry) *apis.FieldError {
	var errs *apis.FieldError
	for i, s := range orderExt {
		errs = errs.Also(validateExtensionGroup(s).ViaIndex(i))
	}
	return errs
}

func validateGroup(group BuilderOrderEntry) *apis.FieldError {
	var errs *apis.FieldError
	for i, s := range group.Group {
		errs = errs.Also(validateBuildpackRef(s).ViaIndex(i).ViaField("group"))
	}
	return errs
}

func validateExtensionGroup(group BuilderOrderEntry) *apis.FieldError {
	var errs *apis.FieldError
	for i, s := range group.Group {
		errs = errs.Also(validateExtensionRef(s).ViaIndex(i).ViaField("group"))
	}
	return errs
}

func validateBuildpackRef(ref BuilderBuildpackRef) *apis.FieldError {
	var errs *apis.FieldError
	if ref.Name != "" || ref.Kind != "" {
		errs = errs.Also(validateObjectRef(ref.ObjectReference, []string{BuildpackKind, ClusterBuildpackKind}))
	}
	errs = errs.Also(validateImage(ref))
	return errs
}

func validateExtensionRef(ref BuilderBuildpackRef) *apis.FieldError {
	var errs *apis.FieldError
	if ref.Name != "" || ref.Kind != "" {
		errs = errs.Also(validateObjectRef(ref.ObjectReference, []string{ExtensionKind, ClusterExtensionKind}))
	}
	errs = errs.Also(validateImage(ref))
	return errs
}

func validateImage(ref BuilderBuildpackRef) *apis.FieldError {
	var errs *apis.FieldError
	switch {
	case ref.Image != "":
		errs = errs.Also(apis.ErrDisallowedFields("image reference currently not supported"))
	case ref.Id != "" || ref.Name != "" || ref.Kind != "":
		if ref.Image != "" {
			errs = errs.Also(apis.ErrDisallowedFields("image"))
		}
	default:
		errs = errs.Also(apis.ErrMissingOneOf("image", "id", "name + kind"))
	}
	return errs
}

func validateObjectRef(ref v1.ObjectReference, kinds []string) *apis.FieldError {
	var errs *apis.FieldError
	if ref.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	}

	for _, k := range kinds {
		if ref.Kind == k {
			return nil
		}
	}
	return errs.Also(apis.ErrInvalidValue(ref.Kind, "kind", "must be one of "+strings.Join(kinds, ", ")))
}
