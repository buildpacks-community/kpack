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
type ClusterBuildpack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterBuildpackSpec `json:"spec"`
	Status BuildpackStatus      `json:"status"`
}

// +k8s:openapi-gen=true
type ClusterBuildpackSpec struct {
	// +listType
	Source            corev1alpha1.ImageSource `json:"source,omitempty"`
	ServiceAccountRef *corev1.ObjectReference  `json:"serviceAccountRef,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true
type ClusterBuildpackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []ClusterBuildpack `json:"items"`
}

func (*ClusterBuildpack) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(ClusterBuildpackKind)
}
