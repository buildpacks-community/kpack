package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const CustomClusterBuilderKind = "CustomClusterBuilder"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object,k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMetaAccessor

type CustomClusterBuilder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CustomClusterBuilderSpec `json:"spec"`
	Status CustomBuilderStatus      `json:"status"`
}

type CustomClusterBuilderSpec struct {
	CustomBuilderSpec
	ServiceAccountRef corev1.ObjectReference `json:"serviceAccountRef"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CustomClusterBuilderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CustomClusterBuilder `json:"items"`
}
