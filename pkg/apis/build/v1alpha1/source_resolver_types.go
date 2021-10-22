package v1alpha1

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type SourceResolver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SourceResolverSpec   `json:"spec"`
	Status            SourceResolverStatus `json:"status,omitempty"`
}

// +k8s:openapi-gen=true
type SourceResolverSpec struct {
	ServiceAccount string                    `json:"serviceAccount,omitempty"`
	Source         corev1alpha1.SourceConfig `json:"source"`
}

// +k8s:openapi-gen=true
type SourceResolverStatus struct {
	corev1alpha1.Status `json:",inline"`
	Source              corev1alpha1.ResolvedSourceConfig `json:"source,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type SourceResolverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []SourceResolver `json:"items"`
}

func (*SourceResolver) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("SourceResolver")
}

func (i *SourceResolver) ConvertTo(_ context.Context, _ apis.Convertible) error {
	return errors.New("called convertTo in non-hub apiVersion v1alpha1")
}

func (i *SourceResolver) ConvertFrom(_ context.Context, _ apis.Convertible) error {
	return errors.New("called convertFrom in non-hub apiVersion v1alpha1")
}
