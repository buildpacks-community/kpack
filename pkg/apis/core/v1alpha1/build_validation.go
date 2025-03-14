package v1alpha1

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	corev1 "k8s.io/api/core/v1"
	"regexp"

	"knative.dev/pkg/apis"
)

func (bbs *BuildBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	var _ *apis.FieldError
	return validateBuilderOrImagePresent(ctx, bbs).
		Also(validateBuilderRefAndImageMutuallyExclusive(ctx, bbs)).
		Also(validateBuilderRef(ctx, bbs.BuilderRef).ViaField("builderRef")).
		Also(validateImage(bbs.Image))

}

func validateImage(value string) *apis.FieldError {
	if value == "" {
		return nil
	}

	_, err := name.ParseReference(value, name.WeakValidation)
	if err != nil {
		return apis.ErrInvalidValue(value, "image")
	}
	return nil
}
func validateBuilderRef(ctx context.Context, ref *corev1.ObjectReference) *apis.FieldError {

	if ref == nil {
		return nil
	}

	var errs *apis.FieldError
	if ref.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	}
	//check for kind
	if ref.Kind == "" {
		errs = errs.Also(apis.ErrMissingField("kind"))
	}

	return errs
}
func validateBuilderRefAndImageMutuallyExclusive(ctx context.Context, bbs *BuildBuilderSpec) *apis.FieldError {
	var errs *apis.FieldError

	if bbs.Image != "" && bbs.BuilderRef != nil {
		//return some err using error.also
		return errs.Also(
			apis.ErrGeneric("image and builderRef fields must be mutually exclusive", "image", "builderRef"),
			//apis.ErrGeneric(
			//	fmt.Sprintf("image and builderRef fields must be mutually exclusive %q%v", bbs.Image, bbs.BuilderRef),
			//),
		)
	}
	return errs
}
func validateBuilderOrImagePresent(ctx context.Context, bbs *BuildBuilderSpec) *apis.FieldError {
	var errs *apis.FieldError
	if bbs.Image == "" && bbs.BuilderRef == nil {
		return errs.Also(
			apis.ErrGeneric("one of the image or builderRef fields must not be empty", "image", "builderRef"),
		)
	}
	return errs
}

func (bs CNBBindings) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	names := map[string]int{}
	for i, b := range bs {
		// check name uniqueness
		if n, ok := names[b.Name]; ok {
			errs = errs.Also(
				apis.ErrGeneric(
					fmt.Sprintf("duplicate binding name %q", b.Name),
					fmt.Sprintf("[%d].name", n),
					fmt.Sprintf("[%d].name", i),
				),
			)
		}
		names[b.Name] = i
		errs = errs.Also(b.Validate(ctx).ViaIndex(i))
	}
	return errs
}

var bindingNameRE = regexp.MustCompile(`^[a-z0-9\-\.]{1,253}$`)

func (b *CNBBinding) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError

	if b.Name == "" {
		errs = errs.Also(apis.ErrMissingField("name"))
	} else if !bindingNameRE.MatchString(b.Name) {
		errs = errs.Also(apis.ErrInvalidValue(b.Name, "name"))
	}

	if b.MetadataRef == nil {
		// metadataRef is required
		errs = errs.Also(apis.ErrMissingField("metadataRef"))
	} else if b.MetadataRef.Name == "" {
		errs = errs.Also(apis.ErrMissingField("metadataRef.name"))
	}

	if b.SecretRef != nil && b.SecretRef.Name == "" {
		// secretRef is optional
		errs = errs.Also(apis.ErrMissingField("secretRef.name"))
	}

	return errs
}
