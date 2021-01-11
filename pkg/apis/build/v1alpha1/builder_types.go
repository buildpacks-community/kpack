package v1alpha1

import (
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const BuilderKind = "Builder"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type Builder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NamespacedBuilderSpec `json:"spec"`
	Status BuilderStatus         `json:"status"`
}

// +k8s:openapi-gen=true
type BuilderSpec struct {
	Tag   string                 `json:"tag,omitempty"`
	Stack corev1.ObjectReference `json:"stack,omitempty"`
	Store corev1.ObjectReference `json:"store,omitempty"`
	// +listType
	Order []OrderEntry `json:"order,omitempty"`
}

// +k8s:openapi-gen=true
type NamespacedBuilderSpec struct {
	BuilderSpec    `json:",inline"`
	ServiceAccount string `json:"serviceAccount,omitempty"`
}

// +k8s:openapi-gen=true
type BuilderStatus struct {
	corev1alpha1.Status     `json:",inline"`
	BuilderMetadata         BuildpackMetadataList `json:"builderMetadata,omitempty"`
	Order                   []OrderEntry          `json:"order,omitempty"`
	Stack                   BuildStack            `json:"stack,omitempty"`
	LatestImage             string                `json:"latestImage,omitempty"`
	ObservedStackGeneration int64                 `json:"observedStackGeneration,omitempty"`
	ObservedStoreGeneration int64                 `json:"observedStoreGeneration,omitempty"`
	OS                      string                `json:"os,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type BuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []Builder `json:"items"`
}

func (*Builder) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(BuilderKind)
}

func (c *Builder) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: c.Namespace, Name: c.Name}
}
