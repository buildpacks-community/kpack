package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

func (n *NotaryConfig) Validate(ctx context.Context) *apis.FieldError {
	if n == nil {
		return nil
	}
	return n.V1.Validate(ctx).ViaField("v1")
}

func (n *NotaryV1Config) Validate(ctx context.Context) *apis.FieldError {
	if n == nil {
		return nil
	}
	return validate.FieldNotEmpty(n.URL, "url").
		Also(validate.FieldNotEmpty(n.SecretRef.Name, "secretRef.name"))
}
