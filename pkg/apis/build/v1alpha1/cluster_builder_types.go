package v1alpha1

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/pkg/apis"
)

const ClusterBuilderKind = "ClusterBuilder"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type ClusterBuilder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterBuilderSpec `json:"spec"`
	Status BuilderStatus      `json:"status"`
}

// +k8s:openapi-gen=true
type ClusterBuilderSpec struct {
	BuilderSpec       `json:",inline"`
	ServiceAccountRef corev1.ObjectReference `json:"serviceAccountRef,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type ClusterBuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []ClusterBuilder `json:"items"`
}

func (*ClusterBuilder) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(ClusterBuilderKind)
}

func (c *ClusterBuilder) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: c.Namespace, Name: c.Name}
}

func (c *ClusterBuilder) ConvertTo(_ context.Context, _ apis.Convertible) error {
	return errors.New("called convertTo in non-hub apiVersion v1alpha1")
}

func (c *ClusterBuilder) ConvertFrom(_ context.Context, _ apis.Convertible) error {
	return errors.New("called convertFrom in non-hub apiVersion v1alpha1")
}
