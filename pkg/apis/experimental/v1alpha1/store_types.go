package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const StoreKind = "Store"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type Store struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StoreSpec   `json:"spec"`
	Status StoreStatus `json:"status"`
}

// +k8s:openapi-gen=true
type StoreSpec struct {
	// +listType
	Sources []StoreImage `json:"sources,omitempty"`
}

// +k8s:openapi-gen=true
type StoreImage struct {
	Image string `json:"image,omitempty"`
}

// +k8s:openapi-gen=true
type StoreStatus struct {
	corev1alpha1.Status `json:",inline"`

	// +listType
	Buildpacks []StoreBuildpack `json:"buildpacks,omitempty"`
}

// +k8s:openapi-gen=true
type StoreBuildpack struct {
	BuildpackInfo `json:",inline"`
	StoreImage    StoreImage `json:"storeImage,omitempty"`
	API           string     `json:"api,omitempty"`
	DiffId        string     `json:"diffId,omitempty"`
	Digest        string     `json:"digest,omitempty"`
	Size          int64      `json:"size,omitempty"`

	// +listType
	Order []OrderEntry `json:"order,omitempty"`
	// +listType
	Stacks []BuildpackStack `json:"stacks,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type StoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +listType
	Items []Store `json:"items"`
}

func (*Store) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(StoreKind)
}
