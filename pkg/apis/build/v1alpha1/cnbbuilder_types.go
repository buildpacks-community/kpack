package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CNBBuilder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CNBBuilderSpec   `json:"spec"`
	Status CNBBuilderStatus `json:"status"`
}

type CNBBuilderSpec struct {
	Image           string                   `json:"image"`
	BuilderMetadata CNBBuildpackMetadataList `json:"builderMetadata"`
}

type CNBBuilderStatus struct {
	duckv1alpha1.Status `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CNBBuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CNBBuilder `json:"items"`
}

func (*CNBBuilder) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CNBBuilder")
}

func (b *CNBBuilder) Ref() v1.ObjectReference {

	gvk := b.GetGroupVersionKind()
	return v1.ObjectReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Namespace:  b.Namespace,
		Name:       b.Name,
	}
}
