package v1alpha2

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

func (s *ClusterExtension) SetDefaults(context.Context) {
}

func (s *ClusterExtension) Validate(ctx context.Context) *apis.FieldError {
	return s.Spec.Validate(ctx).ViaField("spec")
}

func (s *ClusterExtensionSpec) Validate(ctx context.Context) *apis.FieldError {
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
