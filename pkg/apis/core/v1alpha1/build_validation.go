package v1alpha1

import (
	"context"
	"fmt"
	"regexp"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

func (bbs *BuildBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return validate.Image(bbs.Image)
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
