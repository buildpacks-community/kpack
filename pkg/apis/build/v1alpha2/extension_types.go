package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	ExtensionKind   = "Extension"
	ExtensionCRName = "extensions.kpack.io"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type Extension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ExtensionSpec   `json:"spec"`
	Status ExtensionStatus `json:"status"`
}

// +k8s:openapi-gen=true
type ExtensionSpec struct {
	// +listType
	corev1alpha1.ImageSource `json:",inline"`
	ServiceAccountName       string `json:"serviceAccountName,omitempty"`
}

// +k8s:openapi-gen=true
type ExtensionStatus struct {
	corev1alpha1.Status `json:",inline"`

	// +listType
	Extensions []corev1alpha1.BuildpackStatus `json:"extensions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type ExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []Extension `json:"items"`
}

func (*Extension) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(ExtensionKind)
}

func (c *Extension) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: c.Namespace, Name: c.Name}
}
