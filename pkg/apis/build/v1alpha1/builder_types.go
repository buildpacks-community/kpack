package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Builder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuilderSpec   `json:"spec"`
	Status BuilderStatus `json:"status"`
}

type BuilderSpec struct {
	Image        string              `json:"image"`
	UpdatePolicy BuilderUpdatePolicy `json:"updatePolicy"`
}

type BuilderUpdatePolicy string

const (
	Polling  BuilderUpdatePolicy = "polling"
	External BuilderUpdatePolicy = "external"
)

type BuilderStatus struct {
	duckv1alpha1.Status `json:",inline"`
	BuilderMetadata     BuildpackMetadataList `json:"builderMetadata"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type BuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Builder `json:"items"`
}

func (*Builder) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("Builder")
}

func (b *Builder) Ref() v1.ObjectReference {

	gvk := b.GetGroupVersionKind()
	return v1.ObjectReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Namespace:  b.Namespace,
		Name:       b.Name,
	}
}
