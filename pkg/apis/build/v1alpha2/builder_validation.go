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
	if cb.Spec.Lifecycle.Name == "" {
		cb.Spec.Lifecycle.Name = DefaultLifecycleName
	}
	if cb.Spec.Lifecycle.Kind == "" {
		cb.Spec.Lifecycle.Kind = ClusterLifecycleKind
	}
}

func (cb *Builder) Validate(ctx context.Context) *apis.FieldError {
	return cb.Spec.Validate(ctx).ViaField("spec")
}

func (s *BuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return validate.Tag(s.Tag).
		Also(validateStack(s.Stack).ViaField("stack")).
		Also(validateStore(s.Store).ViaField("store")).
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

func validateGroup(group BuilderOrderEntry) *apis.FieldError {
	var errs *apis.FieldError
	for i, s := range group.Group {
		errs = errs.Also(validateBuildpackRef(s).ViaIndex(i).ViaField("group"))
	}
	return errs
}

func validateBuildpackRef(ref BuilderBuildpackRef) *apis.FieldError {
	var errs *apis.FieldError
	if ref.Name != "" || ref.Kind != "" {
		errs = errs.Also(validateObjectRef(ref.ObjectReference, []string{BuildpackKind, ClusterBuildpackKind}))
	}

	switch {
	case ref.Image != "":
		errs = errs.Also(apis.ErrDisallowedFields("image reference currently not supported"))
		// errs = errs.Also(validate.Image(ref.Image)).
		// 	Also(apis.CheckDisallowedFields(ref.BuildpackInfo, v1alpha1.BuildpackInfo{})).
		// 	Also(apis.CheckDisallowedFields(ref.ObjectReference, v1.ObjectReference{}))
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
