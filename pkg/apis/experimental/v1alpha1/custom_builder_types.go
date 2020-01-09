package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

const CustomBuilderKind = "CustomBuilder"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type CustomBuilder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CustomNamespacedBuilderSpec `json:"spec"`
	Status CustomBuilderStatus         `json:"status"`
}

// +k8s:openapi-gen=true
type CustomBuilderSpec struct {
	Tag   string `json:"tag"`
	Stack string `json:"stack"`
	Store string `json:"store"`
	// +listType
	Order []OrderEntry `json:"order"`
}

// +k8s:openapi-gen=true
type CustomNamespacedBuilderSpec struct {
	CustomBuilderSpec `json:",inline"`
	ServiceAccount    string `json:"serviceAccount"`
}

// +k8s:openapi-gen=true
type CustomBuilderStatus struct {
	v1alpha1.BuilderStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type CustomBuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +listType
	Items []CustomBuilder `json:"items"`
}

func (*CustomBuilder) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(CustomBuilderKind)
}

func (c *CustomBuilder) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: c.Namespace, Name: c.Name}
}
