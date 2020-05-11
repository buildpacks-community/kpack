package v1alpha1

import (
	"context"
	"fmt"
	"regexp"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/kmp"

	"github.com/pivotal/kpack/pkg/apis/validate"
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
	return validate.ListNotEmpty(bs.Tags, "tags").
		Also(validate.Tags(bs.Tags)).
		Also(bs.Builder.Validate(ctx).ViaField("builder")).
		Also(bs.Source.Validate(ctx).ViaField("source")).
		Also(bs.Bindings.Validate(ctx).ViaField("bindings")).
		Also(bs.LastBuild.Validate(ctx).ViaField("lastBuild")).
		Also(bs.validateImmutableFields(ctx))
}

func (bs *BuildSpec) validateImmutableFields(ctx context.Context) *apis.FieldError {
	if !apis.IsInUpdate(ctx) {
		return nil
	}

	original := apis.GetBaseline(ctx).(*Build)
	if diff, err := kmp.ShortDiff(&original.Spec, bs); err != nil {
		return &apis.FieldError{
			Message: "Failed to diff Build",
			Paths:   []string{"spec"},
			Details: err.Error(),
		}
	} else if diff != "" {
		return &apis.FieldError{
			Message: "Immutable fields changed (-old +new)",
			Paths:   []string{"spec"},
			Details: diff,
		}
	}
	return nil
}

func (bbs *BuildBuilderSpec) Validate(ctx context.Context) *apis.FieldError {
	return validate.Image(bbs.Image)
}

func (bs Bindings) Validate(ctx context.Context) *apis.FieldError {
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

func (b *Binding) Validate(context context.Context) *apis.FieldError {
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

func (lb *LastBuild) Validate(context context.Context) *apis.FieldError {
	if lb == nil || lb.Image == "" {
		return nil
	}

	return validate.Image(lb.Image)
}
