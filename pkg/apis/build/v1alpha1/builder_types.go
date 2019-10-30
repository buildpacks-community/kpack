package v1alpha1

import (
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
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
	Stack        BuilderStackSpec    `json:"stack"`
	UpdatePolicy BuilderUpdatePolicy `json:"updatePolicy"`
}

type BuilderStackSpec struct {
	RunImage RunImageSpec `json:"runImage"`
}

type RunImageSpec struct {
	Mirrors []Mirror `json:"mirrors"`
}

type Mirror struct {
	Image string `json:"image"`
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
	Stack               BuilderStackStatus    `json:"stack"`
	LatestImage         string                `json:"latestImage"`
}

type BuilderStackStatus struct {
	RunImage RunImageStatus `json:"runImage"`
	ID       string         `json:"id"`
}

type RunImageStatus struct {
	LatestImage string   `json:"latestImage"`
	Mirrors     []Mirror `json:"mirrors"`
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
