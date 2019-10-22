package v1alpha1

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"knative.dev/pkg/apis"
)

func (b *Builder) SetDefaults(ctx context.Context) {
	if b.Spec.UpdatePolicy == "" {
		b.Spec.UpdatePolicy = Polling
	}
}

func (b *Builder) Validate(ctx context.Context) *apis.FieldError {
	return b.Spec.Validate(ctx).ViaField("spec")
}

func (bs *BuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	if validateFieldNotEmpty(bs.Image, "image") != nil {
		return apis.ErrMissingField("image")
	}
	_, err := name.ParseReference(bs.Image, name.WeakValidation)
	if err != nil {
		return apis.ErrInvalidValue(bs.Image, "image")
	}

	return nil
}

func (b *ClusterBuilder) SetDefaults(ctx context.Context) {
	if b.Spec.UpdatePolicy == "" {
		b.Spec.UpdatePolicy = Polling
	}
}

func (b *ClusterBuilder) Validate(ctx context.Context) *apis.FieldError {
	return b.Spec.Validate(ctx).ViaField("spec")
}
