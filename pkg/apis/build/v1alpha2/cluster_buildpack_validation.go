package v1alpha2

import (
	"context"

	"github.com/pivotal/kpack/pkg/apis/validate"
	"knative.dev/pkg/apis"
)

func (s *ClusterBuildpack) SetDefaults(context.Context) {
}

func (s *ClusterBuildpack) Validate(ctx context.Context) *apis.FieldError {
	return s.Spec.Validate(ctx).ViaField("spec")
}

func (s *ClusterBuildpackSpec) Validate(ctx context.Context) *apis.FieldError {
	if s.ServiceAccountRef != nil {
		if s.ServiceAccountRef.Name == "" {
			return apis.ErrMissingField("name").ViaField("serviceAccountRef")
		}
		if s.ServiceAccountRef.Namespace == "" {
			return apis.ErrMissingField("namespace").ViaField("serviceAccountRef")
		}
	}

	return validate.Image(s.Image)
}
