package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	Tag   string  `json:"tag"`
	Stack Stack   `json:"stack"`
	Store Store   `json:"store"`
	Order []Group `json:"order"`
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

type Store struct {
	Image string `json:"image"`
}

type Group struct {
	Group []Buildpack `json:"group"`
}

type Buildpack struct {
	ID      string `json:"id"`
	Version string `json:"version"`

	Optional bool `json:"optional"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CustomBuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CustomBuilder `json:"items"`
}
