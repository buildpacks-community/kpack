package v1alpha1

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"knative.dev/pkg/apis"
)

func (s *Store) SetDefaults(context.Context) {
}

func (s *Store) Validate(ctx context.Context) *apis.FieldError {
	return s.Spec.Validate(ctx).ViaField("spec")
}

func (s *StoreSpec) Validate(ctx context.Context) *apis.FieldError {
	if len(s.Sources) == 0 {
		return apis.ErrMissingField("sources")
	}
	var errors *apis.FieldError = nil
	for i, source := range s.Sources {
		_, err := name.NewDigest(source.Image, name.WeakValidation)
		if err != nil {
			//noinspection GoNilness
			errors = errors.Also(apis.ErrInvalidArrayValue(source, "sources", i))
		}
	}
	return errors
}
