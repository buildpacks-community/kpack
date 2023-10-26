package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	BuildpackKind   = "Buildpack"
	BuildpackCRName = "buildpacks.kpack.io"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type Buildpack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuildpackSpec   `json:"spec"`
	Status BuildpackStatus `json:"status"`
}

// +k8s:openapi-gen=true
type BuildpackSpec struct {
	// +listType
	corev1alpha1.ImageSource `json:",inline"`
	ServiceAccountName       string `json:"serviceAccountName,omitempty"`
}

// +k8s:openapi-gen=true
type BuildpackStatus struct {
	corev1alpha1.Status `json:",inline"`

	// +listType
	Buildpacks []corev1alpha1.BuildpackStatus `json:"buildpacks,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type BuildpackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []Buildpack `json:"items"`
}

func (b *Buildpack) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(BuildpackKind)
}

func (b *Buildpack) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: b.Namespace, Name: b.Name}
}

func (b *Buildpack) ModulesStatus() []corev1alpha1.BuildpackStatus {
	return b.Status.Buildpacks
}

func (b *Buildpack) ServiceAccountName() string {
	return b.Spec.ServiceAccountName
}

func (b *Buildpack) ServiceAccountNamespace() string {
	return b.Namespace
}

func (b *Buildpack) TypeMD() metav1.TypeMeta {
	return b.TypeMeta
}
