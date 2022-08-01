package validate

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/name"
	"knative.dev/pkg/apis"
)

func FieldNotEmpty(value, field string) *apis.FieldError {
	if value == "" {
		return apis.ErrMissingField(field)
	}
	return nil
}

func ListNotEmpty(value []string, field string) *apis.FieldError {
	if len(value) == 0 {
		return apis.ErrMissingField(field)
	}
	return nil
}

func ImmutableField(original, current interface{}, field string) *apis.FieldError {
	if original != current {
		return &apis.FieldError{
			Message: "Immutable field changed",
			Paths:   []string{field},
			Details: fmt.Sprintf("got: %v, want: %v", current, original),
		}
	}
	return nil
}

func Tag(value string) *apis.FieldError {
	if value == "" {
		return apis.ErrMissingField("tag")
	}

	_, err := name.NewTag(value, name.WeakValidation)
	if err != nil {
		return apis.ErrInvalidValue(value, "tag")
	}
	return nil
}

func Tags(tags []string, fieldName string) *apis.FieldError {
	var errors *apis.FieldError = nil
	for i, tag := range tags {
		_, err := name.NewTag(tag, name.WeakValidation)
		if err != nil {
			//noinspection GoNilness
			errors = errors.Also(apis.ErrInvalidArrayValue(tag, fieldName, i))
		}
	}
	return errors
}

func Image(value string) *apis.FieldError {
	if value == "" {
		return apis.ErrMissingField("image")
	}

	_, err := name.ParseReference(value, name.WeakValidation)
	if err != nil {
		return apis.ErrInvalidValue(value, "image")
	}
	return nil
}

func StripComponents(value int64) *apis.FieldError {
	if value >= 0 {
		return nil
	}

	return apis.ErrGeneric(fmt.Sprintf("expected positive integer got %d", value), "stripComponents")
}
