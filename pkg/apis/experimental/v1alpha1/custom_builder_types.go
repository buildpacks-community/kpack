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

type CustomBuilder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CustomNamespacedBuilderSpec `json:"spec"`
	Status CustomBuilderStatus         `json:"status"`
}

type CustomBuilderSpec struct {
	Tag   string       `json:"tag"`
	Stack Stack        `json:"stack"`
	Store string       `json:"store"`
	Order []OrderEntry `json:"order"`
}

type CustomNamespacedBuilderSpec struct {
	CustomBuilderSpec
	ServiceAccount string `json:"serviceAccount"`
}

type CustomBuilderStatus struct {
	v1alpha1.BuilderStatus
}

type Stack struct {
	BaseBuilderImage string `json:"baseBuilderImage"` //todo rename, maybe?
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CustomBuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CustomBuilder `json:"items"`
}

func (*CustomBuilder) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(CustomBuilderKind)
}

func (c *CustomBuilder) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: c.Namespace, Name: c.Name}
}
