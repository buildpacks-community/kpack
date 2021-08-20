package v1alpha2

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"knative.dev/pkg/apis"
)

func (s *ClusterStore) SetDefaults(context.Context) {
}

func (s *ClusterStore) Validate(ctx context.Context) *apis.FieldError {
	return s.Spec.Validate(ctx).ViaField("spec")
}

func (s *ClusterStoreSpec) Validate(ctx context.Context) *apis.FieldError {
	if s.ServiceAccountRef != nil {
		if s.ServiceAccountRef.Name == "" {
			return apis.ErrMissingField("name").ViaField("serviceAccountRef")
		}
		if s.ServiceAccountRef.Namespace == "" {
			return apis.ErrMissingField("namespace").ViaField("serviceAccountRef")
		}
	}

	if len(s.Sources) == 0 {
		return apis.ErrMissingField("sources")
	}
	var errors *apis.FieldError = nil
	for i, source := range s.Sources {
		_, err := name.ParseReference(source.Image, name.WeakValidation)
		if err != nil {
			//noinspection GoNilness
			errors = errors.Also(apis.ErrInvalidArrayValue(source, "sources", i))
		}
	}
	return errors
}
