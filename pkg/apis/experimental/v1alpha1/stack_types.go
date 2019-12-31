package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

const StackKind = "Stack"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

type Stack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackSpec   `json:"spec"`
	Status StackStatus `json:"status"`
}

type StackSpec struct {
	Id         string     `json:"id"`
	BuildImage StackImage `json:"buildImage"`
	RunImage   StackImage `json:"runImage"`
}

type StackImage struct {
	Image string `json:"image"`
}

type StackStatus struct {
	duckv1alpha1.Status `json:",inline"`

	BuildImageRef string `json:"buildImageRef"`
	RunImageRef   string `json:"runImageRef"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type StackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Stack `json:"items"`
}

func (*Stack) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(StackKind)
}
