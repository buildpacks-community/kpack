package v1alpha2

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

func (c *CosignConfig) Validate(ctx context.Context) *apis.FieldError {
	if c == nil {
		return nil
	}

	var err *apis.FieldError

	for i, item := range c.Annotations {
		err = err.Also(item.Validate(ctx).ViaIndex(i).ViaField("annotations"))
	}

	return err
}

func (c *CosignAnnotation) Validate(ctx context.Context) *apis.FieldError {
	if c == nil {
		return nil
	}

	return validate.FieldNotEmpty(c.Name, "name").
		Also(validate.FieldNotEmpty(c.Value, "value"))
}
