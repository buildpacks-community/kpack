package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/pivotal/kpack/pkg/apis/validate"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"
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
	return i.Spec.ValidateSpec(ctx).ViaField("spec").
		Also(i.ValidateMetadata(ctx).ViaField("metadata"))
}

func (i *Image) ValidateMetadata(ctx context.Context) *apis.FieldError {
	return i.validateName(i.Name).ViaField("name")
}

func (i *Image) validateName(imageName string) *apis.FieldError {
	msgs := validation.IsValidLabelValue(imageName)
	if len(msgs) > 0 {
		return &apis.FieldError{
			Message: fmt.Sprintf("invalid image name: %s, name must be a a valid label", imageName),
			Paths:   []string{""},
			Details: strings.Join(msgs, ","),
		}
	}
	return nil
}

func (is *ImageSpec) ValidateSpec(ctx context.Context) *apis.FieldError {
	return is.validateTag(ctx).
		Also(validateBuilder(is.Builder).ViaField("builder")).
		Also(is.Source.Validate(ctx).ViaField("source")).
		Also(is.Build.Validate(ctx).ViaField("build")).
		Also(is.validateCacheSize(ctx)).
		Also(is.Notary.Validate(ctx).ViaField("notary"))
}

func (is *ImageSpec) validateTag(ctx context.Context) *apis.FieldError {
	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Image)
		return validate.ImmutableField(original.Spec.Tag, is.Tag, "tag")
	}

	return validate.Tag(is.Tag)
}

func (is *ImageSpec) validateCacheSize(ctx context.Context) *apis.FieldError {
	if is.CacheSize != nil && ctx.Value(HasDefaultStorageClass) == nil {
		return apis.ErrGeneric("spec.cacheSize cannot be set with no default StorageClass")
	}

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Image)
		if original.Spec.CacheSize != nil && is.CacheSize.Cmp(*original.Spec.CacheSize) < 0 {
			return &apis.FieldError{
				Message: "Field cannot be decreased",
				Paths:   []string{"cacheSize"},
				Details: fmt.Sprintf("current: %v, requested: %v", original.Spec.CacheSize, is.CacheSize),
			}
		}
	}

	return nil
}

func validateBuilder(builder v1.ObjectReference) *apis.FieldError {
	if builder.Name == "" {
		return apis.ErrMissingField("name")
	}

	switch builder.Kind {
	case BuilderKind,
		ClusterBuilderKind:
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
