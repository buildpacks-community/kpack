package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const BuilderKind = "Builder"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

type Builder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BuilderWithSecretsSpec `json:"spec"`
	Status BuilderStatus          `json:"status"`
}

type BuilderSpec struct {
	Image        string              `json:"image"`
	UpdatePolicy BuilderUpdatePolicy `json:"updatePolicy"`
}

type BuilderWithSecretsSpec struct {
	BuilderSpec      `json:",inline"`
	ImagePullSecrets []v1.LocalObjectReference `json:"imagePullSecrets,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,15,rep,name=imagePullSecrets"`
}

type BuilderUpdatePolicy string

const (
	Polling  BuilderUpdatePolicy = "polling"
	External BuilderUpdatePolicy = "external"
)

type BuilderStatus struct {
	duckv1alpha1.Status `json:",inline"`
	BuilderMetadata     BuildpackMetadataList `json:"builderMetadata"`
	LatestImage         string                `json:"latestImage"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type BuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Builder `json:"items"`
}

func (*Builder) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(BuilderKind)
}
