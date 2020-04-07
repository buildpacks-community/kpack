package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const StackKind = "Stack"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type Stack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackSpec   `json:"spec"`
	Status StackStatus `json:"status"`
}

// +k8s:openapi-gen=true
type StackSpec struct {
	Id         string         `json:"id,omitempty"`
	BuildImage StackSpecImage `json:"buildImage,omitempty"`
	RunImage   StackSpecImage `json:"runImage,omitempty"`
}

// +k8s:openapi-gen=true
type StackSpecImage struct {
	Image string `json:"image,omitempty"`
}

// +k8s:openapi-gen=true
type StackStatus struct {
	corev1alpha1.Status `json:",inline"`
	ResolvedStack       `json:",inline"`
}

// +k8s:openapi-gen=true
type ResolvedStack struct {
	Id         string           `json:"id,omitempty"`
	BuildImage StackStatusImage `json:"buildImage,omitempty"`
	RunImage   StackStatusImage `json:"runImage,omitempty"`
	// +listType
	Mixins  []string `json:"mixins,omitempty"`
	UserID  int      `json:"userId,omitempty"`
	GroupID int      `json:"groupId,omitempty"`
}

// +k8s:openapi-gen=true
type StackStatusImage struct {
	LatestImage string `json:"latestImage,omitempty"`
	Image       string `json:"image,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type StackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +listType
	Items []Stack `json:"items"`
}

func (*Stack) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(StackKind)
}
