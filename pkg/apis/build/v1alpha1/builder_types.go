package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const BuilderKind = "Builder"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

// +k8s:openapi-gen=true
type Builder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuilderWithSecretsSpec `json:"spec"`
	Status BuilderStatus          `json:"status,omitempty"`
}

// +k8s:openapi-gen=true
type BuilderWithSecretsSpec struct {
	BuilderSpec `json:",inline"`
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,15,rep,name=imagePullSecrets"`
}

// +k8s:openapi-gen=true
type BuilderSpec struct {
	Image        string              `json:"image"`
	UpdatePolicy BuilderUpdatePolicy `json:"updatePolicy,omitempty"`
}

type BuilderUpdatePolicy string

const (
	Polling  BuilderUpdatePolicy = "polling"
	External BuilderUpdatePolicy = "external"
)

// +k8s:openapi-gen=true
type BuilderStatus struct {
	corev1alpha1.Status `json:",inline"`
	BuilderMetadata     BuildpackMetadataList `json:"builderMetadata,omitempty"`
	Stack               BuildStack            `json:"stack,omitempty"`
	LatestImage         string                `json:"latestImage,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=true
type BuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	// +listType
	Items []Builder `json:"items"`
}

func (*Builder) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(BuilderKind)
}
