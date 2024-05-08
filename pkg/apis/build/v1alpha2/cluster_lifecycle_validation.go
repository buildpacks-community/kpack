package v1alpha2

import (
	"context"

	"knative.dev/pkg/apis"
)

func (s *ClusterLifecycle) SetDefaults(context.Context) {
	// TODO
}

func (s *ClusterLifecycle) Validate(ctx context.Context) *apis.FieldError {
	//return s.Spec.Validate(ctx).ViaField("spec")
	// TODO
	return nil
}
