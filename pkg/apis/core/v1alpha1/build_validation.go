package v1alpha1

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pivotal/kpack/pkg/apis/validate"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func (bbs *BuildBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return validateBuilderOrImagePresent(bbs).
		Also(validateBuilderRefAndImageMutuallyExclusive(bbs)).
		Also(validateBuilderRef(bbs.Ref).ViaField("ref")).
		Also(validateImage(bbs.Image))
}

func validateImage(value string) *apis.FieldError {
	if value != "" {
		_, err := name.ParseReference(value, name.WeakValidation)
		if err != nil {
			return apis.ErrInvalidValue(value, "image")
		}
	}

	return nil
}

func validateBuilderRef(ref *corev1.ObjectReference) *apis.FieldError {
	if ref != nil {
		return validate.FieldNotEmpty(ref.Name, "name").Also(validate.FieldNotEmpty(ref.Kind, "kind"))
	}
	return nil
}

func validateBuilderRefAndImageMutuallyExclusive(bbs *BuildBuilderSpec) *apis.FieldError {
	if bbs.Image != "" && bbs.Ref != nil {
		return apis.ErrMultipleOneOf("image", "ref")
	}
	return nil
}
func validateBuilderOrImagePresent(bbs *BuildBuilderSpec) *apis.FieldError {
	if bbs.Image == "" && bbs.Ref == nil {
		return apis.ErrMissingField("image", "ref")
	}
	return nil
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
