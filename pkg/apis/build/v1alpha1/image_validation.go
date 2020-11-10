package v1alpha1

import (
	"context"
	"fmt"
	v1 "k8s.io/api/core/v1"
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
	return is.validateTag(ctx).
		Also(validateBuilder(is.Builder).ViaField("builder")).
		Also(is.Source.Validate(ctx).ViaField("source")).
		Also(is.Build.Validate(ctx).ViaField("build")).
		Also(is.validateCacheSize(ctx))
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

func (ib *ImageBuild) Validate(ctx context.Context) *apis.FieldError {
	if ib == nil {
		return nil
	}

	return ib.Bindings.Validate(ctx).ViaField("bindings")
}
