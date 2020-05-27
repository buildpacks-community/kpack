package v1alpha1

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

type ImageContextKey string

const (
	HasDefaultStorageClass ImageContextKey = "hasDefaultStorageClass"

	defaultServiceAccount = "default"
)

var (
	defaultFailedBuildHistoryLimit     int64 = 10
	defaultSuccessfulBuildHistoryLimit int64 = 10
	defaultCacheSize                   resource.Quantity
)

func init() {
	defaultCacheSize = resource.MustParse("2G")
}

func (s *Image) SetDefaults(ctx context.Context) {
	if s.Spec.ServiceAccount == "" {
		s.Spec.ServiceAccount = defaultServiceAccount
	}

	if s.Spec.ImageTaggingStrategy == "" {
		s.Spec.ImageTaggingStrategy = BuildNumber
	}

	if s.Spec.FailedBuildHistoryLimit == nil {
		s.Spec.FailedBuildHistoryLimit = &defaultFailedBuildHistoryLimit
	}

	if s.Spec.SuccessBuildHistoryLimit == nil {
		s.Spec.SuccessBuildHistoryLimit = &defaultSuccessfulBuildHistoryLimit
	}

	if s.Spec.CacheSize == nil && ctx.Value(HasDefaultStorageClass) != nil {
		s.Spec.CacheSize = &defaultCacheSize
	}
}

func (s *Image) Validate(ctx context.Context) *apis.FieldError {
	return s.Spec.Validate(ctx).ViaField("spec")
}

func (is *ImageSpec) Validate(ctx context.Context) *apis.FieldError {
	return is.validateTag(ctx).
		Also(validateBuilder(is.Builder).ViaField("builder")).
		Also(is.Source.Validate(ctx).ViaField("source")).
		Also(is.Build.Validate(ctx).ViaField("build"))
}

func (im *ImageSpec) validateTag(ctx context.Context) *apis.FieldError {
	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Image)
		return validate.ImmutableField(original.Spec.Tag, im.Tag, "tag")
	}

	return validate.Tag(im.Tag)
}

func validateBuilder(builder v1.ObjectReference) *apis.FieldError {
	if builder.Name == "" {
		return apis.ErrMissingField("name")
	}

	switch builder.Kind {
	case ClusterBuilderKind,
		BuilderKind,
		"CustomBuilder", // TODO : use the const var when the experimental pkg migrates into the build pkg
		"CustomClusterBuilder":
		return nil
	default:
		return apis.ErrInvalidValue(builder.Kind, "kind")
	}
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

func (ib *ImageBuild) Validate(ctx context.Context) *apis.FieldError {
	if ib == nil {
		return nil
	}

	return ib.Bindings.Validate(ctx).ViaField("bindings")
}
