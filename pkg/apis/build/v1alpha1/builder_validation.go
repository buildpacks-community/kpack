package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
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
	return validate.Image(bs.Image)
}

func (b *ClusterBuilder) SetDefaults(ctx context.Context) {
	if b.Spec.UpdatePolicy == "" {
		b.Spec.UpdatePolicy = Polling
	}
}

func (b *ClusterBuilder) Validate(ctx context.Context) *apis.FieldError {
	return b.Spec.Validate(ctx).ViaField("spec")
}
