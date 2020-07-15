package v1alpha1

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

func (s *ClusterStack) SetDefaults(context.Context) {
}

func (s *ClusterStack) Validate(ctx context.Context) *apis.FieldError {
	return s.Spec.Validate(ctx).ViaField("spec")
}

func (ss *ClusterStackSpec) Validate(ctx context.Context) *apis.FieldError {
	return validate.FieldNotEmpty(ss.Id, "id").
		Also(ss.BuildImage.Validate(ctx).ViaField("buildImage")).
		Also(ss.RunImage.Validate(ctx).ViaField("runImage"))
}

func (ssi *ClusterStackSpecImage) Validate(context.Context) *apis.FieldError {
	return validate.Image(ssi.Image)
}
