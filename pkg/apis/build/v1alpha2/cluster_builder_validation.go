package v1alpha2

import (
	"context"

	"knative.dev/pkg/apis"
)

func (ccb *ClusterBuilder) SetDefaults(context.Context) {
	if ccb.Spec.Lifecycle.Name == "" {
		ccb.Spec.Lifecycle.Name = DefaultLifecycleName
	}
	if ccb.Spec.Lifecycle.Kind == "" {
		ccb.Spec.Lifecycle.Kind = ClusterLifecycleKind
	}
}

func (ccb *ClusterBuilder) Validate(ctx context.Context) *apis.FieldError {
	return ccb.Spec.Validate(ctx)
}

func (ccbs *ClusterBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	if ccbs.ServiceAccountRef.Name == "" {
		return apis.ErrMissingField("name").ViaField("spec", "serviceAccountRef")
	}
	if ccbs.ServiceAccountRef.Namespace == "" {
		return apis.ErrMissingField("namespace").ViaField("spec", "serviceAccountRef")
	}
	return ccbs.BuilderSpec.Validate(ctx)
}
