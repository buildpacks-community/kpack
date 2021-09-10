package v1alpha2

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation"
	"knative.dev/pkg/apis"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/apis/validate"
)

type ImageContextKey string

const (
	HasDefaultStorageClass ImageContextKey = "hasDefaultStorageClass"
	IsExpandable           ImageContextKey = "isExpandable"
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
		i.Spec.ImageTaggingStrategy = corev1alpha1.BuildNumber
	}

	if i.Spec.FailedBuildHistoryLimit == nil {
		i.Spec.FailedBuildHistoryLimit = &defaultFailedBuildHistoryLimit
	}

	if i.Spec.SuccessBuildHistoryLimit == nil {
		i.Spec.SuccessBuildHistoryLimit = &defaultSuccessfulBuildHistoryLimit
	}

	if i.Spec.Cache == nil && ctx.Value(HasDefaultStorageClass) != nil {
		i.Spec.Cache = &ImageCacheConfig{
			Volume: &ImagePersistentVolumeCache{
				Size: &defaultCacheSize,
			},
		}
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
		Also(is.Cache.Validate(ctx).ViaField("cache")).
		Also(is.validateVolumeCache(ctx)).
		Also(is.Notary.Validate(ctx).ViaField("notary"))
}

func (is *ImageSpec) validateTag(ctx context.Context) *apis.FieldError {
	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Image)
		return validate.ImmutableField(original.Spec.Tag, is.Tag, "tag")
	}

	return validate.Tag(is.Tag)
}

func (is *ImageSpec) validateVolumeCache(ctx context.Context) *apis.FieldError {
	if is.Cache.Volume != nil && ctx.Value(HasDefaultStorageClass) == nil {
		return apis.ErrGeneric("spec.cache.volume.size cannot be set with no default StorageClass")
	}

	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Image)
		if original.Spec.NeedVolumeCache() && is.NeedVolumeCache() {
			if ctx.Value(IsExpandable) == false && is.Cache.Volume.Size.Cmp(*original.Spec.Cache.Volume.Size) != 0 {
				return &apis.FieldError{
					Message: "Field cannot be changed, default storage class is not expandable",
					Paths:   []string{"cache.volume.size"},
					Details: fmt.Sprintf("current: %v, requested: %v", original.Spec.Cache.Volume.Size, is.Cache.Volume.Size),
				}
			} else if is.Cache.Volume.Size.Cmp(*original.Spec.Cache.Volume.Size) < 0 {
				return &apis.FieldError{
					Message: "Field cannot be decreased",
					Paths:   []string{"cache.volume.size"},
					Details: fmt.Sprintf("current: %v, requested: %v", original.Spec.Cache.Volume.Size, is.Cache.Volume.Size),
				}
			}
		}
	}

	return nil
}

func (ib *ImageBuild) Validate(ctx context.Context) *apis.FieldError {
	if ib == nil {
		return nil
	}

	return ib.Services.Validate(ctx).ViaField("services").
		Also(validateCnbBindingsEmpty(ib.CnbBindings).ViaField("cnbBindings"))
}

func validateCnbBindingsEmpty(bindings corev1alpha1.CnbBindings) *apis.FieldError {
	if len(bindings) > 0 {
		return apis.ErrDisallowedFields("")
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

func (c *ImageCacheConfig) Validate(context context.Context) *apis.FieldError {
	if c != nil && c.Volume != nil && c.Registry != nil {
		return apis.ErrGeneric("only one type of cache can be specified", "volume", "registry")
	}

	return nil
}
