package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	ClusterBuildpackKind   = "ClusterBuildpack"
	ClusterBuildpackCRName = "clusterbuildpacks.kpack.io"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type ClusterExtension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterExtensionSpec   `json:"spec"`
	Status ClusterExtensionStatus `json:"status"`
}

// +k8s:openapi-gen=true
type ClusterExtensionSpec struct {
	// +listType
	corev1alpha1.ImageSource `json:",inline"`
	ServiceAccountRef        *corev1.ObjectReference `json:"serviceAccountRef,omitempty"`
}

// +k8s:openapi-gen=true
type ClusterExtensionStatus struct {
	corev1alpha1.Status `json:",inline"`

	// +listType
	Extensions []corev1alpha1.BuildpackStatus `json:"extensions,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type ClusterExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []ClusterExtension `json:"items"`
}

func (*ClusterExtension) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(ClusterExtensionKind)
}
