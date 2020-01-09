package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kpackcore "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
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
	Sources []StoreImage `json:"sources"`
}

// +k8s:openapi-gen=true
type StoreImage struct {
	Image string `json:"image"`
}

// +k8s:openapi-gen=true
type StoreStatus struct {
	kpackcore.Status `json:",inline"`

	// +listType
	Buildpacks []StoreBuildpack `json:"buildpacks"`
}

// +k8s:openapi-gen=true
type StoreBuildpack struct {
	BuildpackInfo `json:",inline"`
	StoreImage    StoreImage `json:"storeImage"`
	// +listType
	Order  []OrderEntry `json:"order"`
	DiffId string       `json:"diffId"`
	Digest string       `json:"digest"`
	Size   int64        `json:"size"`
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
