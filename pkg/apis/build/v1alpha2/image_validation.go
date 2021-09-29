package v1alpha2

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
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
	if i.Spec.ServiceAccountName == "" {
		i.Spec.ServiceAccountName = "default"
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
		Also(is.validateAdditionalTags(ctx)).
		Also(validateBuilder(is.Builder).ViaField("builder")).
		Also(is.Source.Validate(ctx).ViaField("source")).
		Also(is.Build.Validate(ctx).ViaField("build")).
		Also(is.Cache.Validate(ctx).ViaField("cache")).
		Also(is.validateVolumeCache(ctx)).
		Also(validateNotary(ctx, is.Notary).ViaField("notary")).
		Also(is.Cosign.Validate(ctx).ViaField("cosign")).
		Also(is.validateBuildHistoryLimit())
}

func (is *ImageSpec) validateTag(ctx context.Context) *apis.FieldError {
	if apis.IsInUpdate(ctx) {
		original := apis.GetBaseline(ctx).(*Image)
		return validate.ImmutableField(original.Spec.Tag, is.Tag, "tag")
	}

	return validate.Tag(is.Tag)
}

func (is *ImageSpec) validateAdditionalTags(ctx context.Context) *apis.FieldError {
	return validate.Tags(is.AdditionalTags, "additionalTags").Also(is.validateSameRegistry())
}

func (is *ImageSpec) validateSameRegistry() *apis.FieldError {
	tag, err := name.NewTag(is.Tag, name.WeakValidation)
	// We only care about the non-nil error cases here as we validate
	// the tag validity in other methods which should display appropriate errors.
	if err == nil {
		for _, t := range is.AdditionalTags {
			addT, err := name.NewTag(t, name.WeakValidation)
			if err == nil {
				if addT.RegistryStr() != tag.RegistryStr() {
					return &apis.FieldError{
						Message: "all additionalTags must have the same registry as tag",
						Paths:   []string{"additionalTags"},
						Details: fmt.Sprintf("expected registry: %s, got: %s", tag.RegistryStr(), addT.RegistryStr()),
					}
				}
			}
		}
	}
	return nil
}

func (is *ImageSpec) validateVolumeCache(ctx context.Context) *apis.FieldError {
	if is.Cache != nil && is.Cache.Volume != nil && ctx.Value(HasDefaultStorageClass) == nil {
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

	if len(ib.NodeSelector) != 0 {
		if _, ok := ib.NodeSelector[k8sOSLabel]; ok {
			return apis.ErrInvalidKeyName(k8sOSLabel, "nodeSelector", "os is determined automatically")
		}
	}

	return ib.Services.Validate(ctx).ViaField("services").
		Also(validateCnbBindings(ctx, ib.CNBBindings).ViaField("cnbBindings"))
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

func (is *ImageSpec) validateBuildHistoryLimit() *apis.FieldError {
	errMsg := "build history limit must be greater than 0"

	if *is.FailedBuildHistoryLimit < 1 {
		return apis.ErrGeneric(errMsg, "failedBuildHistoryLimit")
	}
	if *is.SuccessBuildHistoryLimit < 1 {
		return apis.ErrGeneric(errMsg, "successBuildHistoryLimit")
	}
	return nil
}

func (c *ImageCacheConfig) Validate(context context.Context) *apis.FieldError {
	if c != nil && c.Volume != nil && c.Registry != nil {
		return apis.ErrGeneric("only one type of cache can be specified", "volume", "registry")
	}

	return nil
}

func validateNotary(ctx context.Context, config *corev1alpha1.NotaryConfig) *apis.FieldError {
	//only allow the kpack controller to create resources with notary
	if !resourceCreatedByKpackController(apis.GetUserInfo(ctx)) && config != nil {
		return apis.ErrGeneric("use of this field has been deprecated in v1alpha2, please use v1alpha1 for notary image signing", "")
	}

	return config.Validate(ctx)
}

func validateCnbBindings(ctx context.Context, bindings corev1alpha1.CNBBindings) *apis.FieldError {
	//only allow the kpack controller to create resources with cnb bindings
	if !resourceCreatedByKpackController(apis.GetUserInfo(ctx)) && len(bindings) > 0 {
		return apis.ErrGeneric("use of this field has been deprecated in v1alpha2, please use v1alpha1 for CNB bindings", "")
	}

	return bindings.Validate(ctx)
}
