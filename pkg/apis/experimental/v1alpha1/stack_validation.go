package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

func (s *Stack) SetDefaults(context.Context) {
}

func (s *Stack) Validate(ctx context.Context) *apis.FieldError {
	return s.Spec.Validate(ctx).ViaField("spec")
}

func (ss *StackSpec) Validate(ctx context.Context) *apis.FieldError {
	return validate.FieldNotEmpty(ss.Id, "id").
		Also(ss.BuildImage.Validate(ctx).ViaField("buildImage")).
		Also(ss.RunImage.Validate(ctx).ViaField("runImage"))
}

func (ssi *StackSpecImage) Validate(context.Context) *apis.FieldError {
	return validate.Image(ssi.Image)
}
