package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

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

	Spec   ClusterBuildpackSpec   `json:"spec"`
	Status ClusterBuildpackStatus `json:"status"`
}

// +k8s:openapi-gen=true
type ClusterBuildpackSpec struct {
	// +listType
	corev1alpha1.ImageSource `json:",inline"`
	ServiceAccountRef        *corev1.ObjectReference `json:"serviceAccountRef,omitempty"`
}

// +k8s:openapi-gen=true
type ClusterBuildpackStatus struct {
	corev1alpha1.Status `json:",inline"`

	// +listType
	Buildpacks []corev1alpha1.BuildpackStatus `json:"buildpacks,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type ClusterBuildpackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []ClusterBuildpack `json:"items"`
}

func (b *ClusterBuildpack) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(ClusterBuildpackKind)
}

func (b *ClusterBuildpack) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: b.Namespace, Name: b.Name}
}

func (b *ClusterBuildpack) ModulesStatus() []corev1alpha1.BuildpackStatus {
	return b.Status.Buildpacks
}

func (b *ClusterBuildpack) ServiceAccountName() string {
	if b.Spec.ServiceAccountRef == nil {
		return ""
	}
	return b.Spec.ServiceAccountRef.Name
}

func (b *ClusterBuildpack) ServiceAccountNamespace() string {
	if b.Spec.ServiceAccountRef == nil {
		return ""
	}
	return b.Spec.ServiceAccountRef.Namespace
}

func (b *ClusterBuildpack) TypeMD() metav1.TypeMeta {
	return b.TypeMeta
}
