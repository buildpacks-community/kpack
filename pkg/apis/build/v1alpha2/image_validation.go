package v1alpha2

import (
	"context"

	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

type ImageContextKey string

const (
	HasDefaultStorageClass ImageContextKey = "hasDefaultStorageClass"
)

var (
	defaultFailedBuildHistoryLimit     int64 = 10
	defaultSuccessfulBuildHistoryLimit int64 = 10
	defaultCacheSize                   resource.Quantity
)

func init() {
	defaultCacheSize = resource.MustParse("2G")
}

func (i *Image) SetDefaults(ctx context.Context) {
	if i.Spec.ServiceAccount == "" {
		i.Spec.ServiceAccount = "default"
	}

	if i.Spec.ImageTaggingStrategy == "" {
		i.Spec.ImageTaggingStrategy = BuildNumber
	}

	if i.Spec.FailedBuildHistoryLimit == nil {
		i.Spec.FailedBuildHistoryLimit = &defaultFailedBuildHistoryLimit
	}

	if i.Spec.SuccessBuildHistoryLimit == nil {
		i.Spec.SuccessBuildHistoryLimit = &defaultSuccessfulBuildHistoryLimit
	}

	if i.Spec.CacheSize == nil && ctx.Value(HasDefaultStorageClass) != nil {
		i.Spec.CacheSize = &defaultCacheSize
	}
}

func (i *Image) Validate(ctx context.Context) *apis.FieldError {
	return i.Spec.Validate(ctx).ViaField("spec")
}

func (is *ImageSpec) Validate(ctx context.Context) *apis.FieldError {
	return is.Validate(ctx).
		Also(is.Build.Validate(ctx).ViaField("build"))
}

func (ib *ImageBuild) Validate(ctx context.Context) *apis.FieldError {
	if ib == nil {
		return nil
	}

	return ib.Services.Validate(ctx).ViaField("services")
}

func (s *SourceConfig) Validate(ctx context.Context) *apis.FieldError {
	sources := make([]string, 0, 3)
	if s.Git != nil {
		sources = append(sources, "git")
	}
	if s.Blob != nil {
		sources = append(sources, "blob")
	}
	if s.Registry != nil {
		sources = append(sources, "registry")
	}

	if len(sources) == 0 {
		return apis.ErrMissingOneOf("git", "blob", "registry")
	}

	if len(sources) != 1 {
		return apis.ErrMultipleOneOf(sources...)
	}

	return (s.Git.Validate(ctx).ViaField("git")).
		Also(s.Blob.Validate(ctx).ViaField("blob")).
		Also(s.Registry.Validate(ctx).ViaField("registry"))
}

func (g *Git) Validate(ctx context.Context) *apis.FieldError {
	if g == nil {
		return nil
	}

	return validate.FieldNotEmpty(g.URL, "url").
		Also(validate.FieldNotEmpty(g.Revision, "revision"))
}

func (b *Blob) Validate(ctx context.Context) *apis.FieldError {
	if b == nil {
		return nil
	}

	return validate.FieldNotEmpty(b.URL, "url")
}

func (r *Registry) Validate(ctx context.Context) *apis.FieldError {
	if r == nil {
		return nil
	}

	return validate.Image(r.Image)
}
