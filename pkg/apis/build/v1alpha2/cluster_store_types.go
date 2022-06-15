package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	ClusterStoreKind   = "ClusterStore"
	ClusterStoreCRName = "clusterstores.kpack.io"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type ClusterStore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterStoreSpec   `json:"spec"`
	Status ClusterStoreStatus `json:"status"`
}

// +k8s:openapi-gen=true
type ClusterStoreSpec struct {
	// +listType
	Sources           []corev1alpha1.StoreImage `json:"sources,omitempty"`
	ServiceAccountRef *corev1.ObjectReference   `json:"serviceAccountRef,omitempty"`
}

// +k8s:openapi-gen=true
type ClusterStoreStatus struct {
	corev1alpha1.Status `json:",inline"`

	// +listType
	Buildpacks []corev1alpha1.StoreBuildpack `json:"buildpacks,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type ClusterStoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []ClusterStore `json:"items"`
}

func (*ClusterStore) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(ClusterStoreKind)
}
