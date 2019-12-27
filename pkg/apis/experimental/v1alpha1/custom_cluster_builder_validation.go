package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"
)

func (ccb *CustomClusterBuilder) SetDefaults(context.Context) {
}

func (ccb *CustomClusterBuilder) Validate(ctx context.Context) *apis.FieldError {
	return ccb.Spec.Validate(ctx)
}

func (ccbs *CustomClusterBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	if ccbs.ServiceAccountRef.Name == "" {
		return apis.ErrMissingField("name").ViaField("spec", "serviceAccountRef")
	}
	if ccbs.ServiceAccountRef.Namespace == "" {
		return apis.ErrMissingField("namespace").ViaField("spec", "serviceAccountRef")
	}
	return ccbs.CustomBuilderSpec.Validate(ctx)
}
