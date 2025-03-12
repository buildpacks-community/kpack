package v1alpha2

import (
	"context"

	"knative.dev/pkg/apis"

	"github.com/pivotal/kpack/pkg/apis/validate"
)

func (cl *ClusterLifecycle) SetDefaults(context.Context) {
}

func (cl *ClusterLifecycle) Validate(ctx context.Context) *apis.FieldError {
	return cl.Spec.Validate(ctx).ViaField("spec")
}

func (cls *ClusterLifecycleSpec) Validate(ctx context.Context) *apis.FieldError {
	if cls.ServiceAccountRef != nil {
		if cls.ServiceAccountRef.Name == "" {
			return apis.ErrMissingField("name").ViaField("serviceAccountRef")
		}
		if cls.ServiceAccountRef.Namespace == "" {
			return apis.ErrMissingField("namespace").ViaField("serviceAccountRef")
		}
	}

	return validate.FieldNotEmpty(cls.Image, "image")
}
