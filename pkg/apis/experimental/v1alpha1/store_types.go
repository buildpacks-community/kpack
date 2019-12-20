package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

const StoreKind = "Store"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

type Store struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StoreSpec   `json:"spec"`
	Status StoreStatus `json:"status"`
}

type StoreSpec struct {
	Sources []StoreImage `json:"sources"`
}

type StoreImage struct {
	Image string `json:"image"`
}

type StoreStatus struct {
	duckv1alpha1.Status `json:",inline"`
	Buildpacks          []StoreBuildpack `json:"buildpacks"`
}

type StoreBuildpack struct {
	BuildpackInfo
	LayerDiffID string       `json:"layerDiffId"`
	StoreImage  StoreImage   `json:"storeImage"`
	Order       []OrderEntry `json:"order"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type StoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Store `json:"items"`
}

func (*Store) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(StoreKind)
}
