package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	ClusterExtensionKind   = "ClusterExtension"
	ClusterExtensionCRName = "clusterextensions.kpack.io"
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

func (e *ClusterExtension) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(ClusterExtensionKind)
}

func (e *ClusterExtension) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: e.Namespace, Name: e.Name}
}

func (e *ClusterExtension) ModulesStatus() []corev1alpha1.BuildpackStatus {
	return e.Status.Extensions
}

func (e *ClusterExtension) ServiceAccountName() string {
	if e.Spec.ServiceAccountRef == nil {
		return ""
	}
	return e.Spec.ServiceAccountRef.Name
}

func (e *ClusterExtension) ServiceAccountNamespace() string {
	if e.Spec.ServiceAccountRef == nil {
		return ""
	}
	return e.Spec.ServiceAccountRef.Namespace
}

func (e *ClusterExtension) TypeMD() metav1.TypeMeta {
	return e.TypeMeta
}
