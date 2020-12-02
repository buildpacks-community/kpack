package v1alpha2

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
		b.Spec.ServiceAccount = "default"
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
		Also(bs.Services.Validate(ctx).ViaField("services")).
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

var serviceNameRE = regexp.MustCompile(`^[a-z0-9\-\.]{1,253}$`)

func (ss Services) Validate(ctx context.Context) *apis.FieldError {
	var errs *apis.FieldError
	names := map[string]int{}
	for i, s := range ss {
		// check name uniqueness
		if n, ok := names[s.Name]; ok {
			errs = errs.Also(
				apis.ErrGeneric(
					fmt.Sprintf("duplicate service name %q", s.Name),
					fmt.Sprintf("[%d].name", n),
					fmt.Sprintf("[%d].name", i),
				),
			)
		}
		names[s.Name] = i
		if s.Name == "" {
			errs = errs.Also(apis.ErrMissingField("name").ViaIndex(i))
		} else if !serviceNameRE.MatchString(s.Name) {
			errs = errs.Also(apis.ErrInvalidValue(s.Name, "name").ViaIndex(i))
		}
	}
	return errs
}

func (lb *LastBuild) Validate(context context.Context) *apis.FieldError {
	if lb == nil || lb.Image == "" {
		return nil
	}

	return validate.Image(lb.Image)
}
