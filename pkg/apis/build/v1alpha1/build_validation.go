package v1alpha1

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"knative.dev/pkg/apis"
)

func (b *Build) SetDefaults(ctx context.Context) {
	if b.Spec.ServiceAccount == "" {
		b.Spec.ServiceAccount = defaultServiceAccount
	}
}

func (b *Build) Validate(ctx context.Context) *apis.FieldError {
	return b.Spec.Validate(ctx).ViaField("spec")
}

func (bs *BuildSpec) Validate(ctx context.Context) *apis.FieldError {
	return validateListNotEmpty(bs.Tags, "tags").
		Also(bs.Builder.Validate(ctx).ViaField("builder")).
		Also(bs.Source.Validate(ctx).ViaField("source"))
}

func (bbs *BuildBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	if bbs.Image == "" {
		return apis.ErrMissingField("name")
	}

	_, err := name.ParseReference(bbs.Image, name.WeakValidation)
	if err != nil {
		return apis.ErrInvalidValue(bbs.Image, "image")
	}

	switch bbs.Kind {
	case ClusterBuilderKind,
		BuilderKind:
		return nil
	default:
		return apis.ErrInvalidValue(bbs.Kind, "kind")
	}
}
