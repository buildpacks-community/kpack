package v1alpha1

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type ImageBuild struct {
	// +listType
	Bindings Bindings `json:"bindings,omitempty"`
	// +listType
	Env       []corev1.EnvVar             `json:"env,omitempty"`
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

type ImageTaggingStrategy string

const (
	None        ImageTaggingStrategy = "None"
	BuildNumber ImageTaggingStrategy = "BuildNumber"
)

func (ib *ImageBuild) Validate(ctx context.Context) *apis.FieldError {
	if ib == nil {
		return nil
	}

	return ib.Bindings.Validate(ctx).ViaField("bindings")
}
