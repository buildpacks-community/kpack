package v1alpha1

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func (ccb *CustomClusterBuilder) SetDefaults(context.Context) {
}

func (ccb *CustomClusterBuilder) Validate(ctx context.Context) *apis.FieldError {
	return ccb.Spec.CustomBuilderSpec.Validate(ctx).
		Also(validateServiceAccountRef(ccb.Spec.ServiceAccountRef))
}

func validateServiceAccountRef(serviceAccount v1.ObjectReference) *apis.FieldError {
	if serviceAccount.Name == "" {
		return apis.ErrMissingField("name").ViaField("spec", "serviceAccountRef")
	}
	if serviceAccount.Namespace == "" {
		return apis.ErrMissingField("namespace").ViaField("spec", "serviceAccountRef")
	}
	return nil
}
