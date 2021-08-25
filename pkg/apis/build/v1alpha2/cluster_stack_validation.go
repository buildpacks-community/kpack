package v1alpha2

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
	if ss.ServiceAccountRef != nil {
		if ss.ServiceAccountRef.Name == "" {
			return apis.ErrMissingField("name").ViaField("serviceAccountRef")
		}
		if ss.ServiceAccountRef.Namespace == "" {
			return apis.ErrMissingField("namespace").ViaField("serviceAccountRef")
		}
	}

	return validate.FieldNotEmpty(ss.Id, "id").
		Also(ss.BuildImage.Validate(ctx).ViaField("buildImage")).
		Also(ss.RunImage.Validate(ctx).ViaField("runImage"))
}

func (ssi *ClusterStackSpecImage) Validate(context.Context) *apis.FieldError {
	return validate.Image(ssi.Image)
}
