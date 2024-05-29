package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	ClusterLifecycleKind   = "ClusterLifecycle"
	ClusterLifecycleCRName = "clusterlifecycles.kpack.io"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type ClusterLifecycle struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterLifecycleSpec   `json:"spec"`
	Status ClusterLifecycleStatus `json:"status"`
}

// +k8s:openapi-gen=true
type ClusterLifecycleSpec struct {
	// +listType
	corev1alpha1.ImageSource `json:",inline"`
	ServiceAccountRef        *corev1.ObjectReference `json:"serviceAccountRef,omitempty"`
}

// +k8s:openapi-gen=true
type ClusterLifecycleStatus struct {
	corev1alpha1.Status      `json:",inline"`
	ResolvedClusterLifecycle `json:",inline"`
}

// +k8s:openapi-gen=true
type ResolvedClusterLifecycle struct {
	Id      string `json:"id,omitempty"` // TODO: should this be LatestImage?
	Version string `json:"version,omitempty"`

	// Deprecated: Use `LifecycleAPIs` instead
	API  LifecycleAPI  `json:"api,omitempty"`
	APIs LifecycleAPIs `json:"apis,omitempty"`
}

type LifecycleAPI struct {
	BuildpackVersion string `toml:"buildpack" json:"buildpack,omitempty"`
	PlatformVersion  string `toml:"platform" json:"platform,omitempty"`
}

type LifecycleAPIs struct {
	Buildpack APIVersions `toml:"buildpack" json:"buildpack"`
	Platform  APIVersions `toml:"platform" json:"platform"`
}

type APIVersions struct {
	Deprecated APISet `toml:"deprecated" json:"deprecated"`
	Supported  APISet `toml:"supported" json:"supported"`
}

type APISet []string

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type ClusterLifecycleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +k8s:listType=atomic
	Items []ClusterLifecycle `json:"items"`
}

func (*ClusterLifecycle) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(ClusterLifecycleKind)
}
