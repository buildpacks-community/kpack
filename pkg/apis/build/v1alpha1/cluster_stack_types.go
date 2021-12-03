package v1alpha1

import (
	"context"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const ClusterStackKind = "ClusterStack"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type ClusterStack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterStackSpec   `json:"spec"`
	Status ClusterStackStatus `json:"status"`
}

// +k8s:openapi-gen=true
type ClusterStackSpec struct {
	Id         string                `json:"id,omitempty"`
	BuildImage ClusterStackSpecImage `json:"buildImage,omitempty"`
	RunImage   ClusterStackSpecImage `json:"runImage,omitempty"`
}

// +k8s:openapi-gen=true
type ClusterStackSpecImage struct {
	Image string `json:"image,omitempty"`
}

// +k8s:openapi-gen=true
type ClusterStackStatus struct {
	corev1alpha1.Status  `json:",inline"`
	ResolvedClusterStack `json:",inline"`
}

// +k8s:openapi-gen=true
type ResolvedClusterStack struct {
	Id         string                  `json:"id,omitempty"`
	BuildImage ClusterStackStatusImage `json:"buildImage,omitempty"`
	RunImage   ClusterStackStatusImage `json:"runImage,omitempty"`
	// +listType
	Mixins  []string `json:"mixins,omitempty"`
	UserID  int      `json:"userId,omitempty"`
	GroupID int      `json:"groupId,omitempty"`
}

// +k8s:openapi-gen=true
type ClusterStackStatusImage struct {
	LatestImage string `json:"latestImage,omitempty"`
	Image       string `json:"image,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type ClusterStackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []ClusterStack `json:"items"`
}

func (*ClusterStack) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(ClusterStackKind)
}

func (s *ClusterStack) ConvertTo(_ context.Context, _ apis.Convertible) error {
	return errors.New("called convertTo in non-hub apiVersion v1alpha1")
}

func (s *ClusterStack) ConvertFrom(_ context.Context, _ apis.Convertible) error {
	return errors.New("called convertFrom in non-hub apiVersion v1alpha1")
}
