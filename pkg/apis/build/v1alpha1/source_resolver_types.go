package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SourceResolver struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SourceResolverSpec   `json:"spec"`
	Status            SourceResolverStatus `json:"status"`
}

type SourceResolverSpec struct {
	ServiceAccount string       `json:"serviceAccount"`
	Source         SourceConfig `json:"source"`
}

type SourceResolverStatus struct {
	duckv1alpha1.Status `json:",inline"`
	Source              ResolvedSourceConfig `json:"source"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type SourceResolverList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []SourceResolver `json:"items"`
}

func (*SourceResolver) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("SourceResolver")
}
