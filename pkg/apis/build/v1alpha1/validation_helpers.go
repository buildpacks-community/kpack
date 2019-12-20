package v1alpha1

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"knative.dev/pkg/apis"
)

func ValidateFieldNotEmpty(value, field string) *apis.FieldError {
	if value == "" {
		return apis.ErrMissingField(field)
	}
	return nil
}

func ValidateListNotEmpty(value []string, field string) *apis.FieldError {
	if len(value) == 0 {
		return apis.ErrMissingField(field)
	}
	return nil
}

func ValidateImmutableField(original, current interface{}, field string) *apis.FieldError {
	if original != current {
		return &apis.FieldError{
			Message: "Immutable field changed",
			Paths:   []string{field},
			Details: fmt.Sprintf("got: %v, want: %v", current, original),
		}
	}
	return nil
}

func ValidateTag(value string) *apis.FieldError {
	if value == "" {
		return apis.ErrMissingField("tag")
	}

	_, err := name.NewTag(value, name.WeakValidation)
	if err != nil {
		return apis.ErrInvalidValue(value, "tag")
	}
	return nil
}

func ValidateTags(tags []string) *apis.FieldError {
	var errors *apis.FieldError = nil
	for i, tag := range tags {
		_, err := name.NewTag(tag, name.WeakValidation)
		if err != nil {
			//noinspection GoNilness
			errors = errors.Also(apis.ErrInvalidArrayValue(tag, "tags", i))
		}
	}
	return errors
}

func ValidateImage(value string) *apis.FieldError {
	if value == "" {
		return apis.ErrMissingField("image")
	}

	_, err := name.ParseReference(value, name.WeakValidation)
	if err != nil {
		return apis.ErrInvalidValue(value, "image")
	}
	return nil
}
