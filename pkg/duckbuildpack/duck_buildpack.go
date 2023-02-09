package duckbuildpack

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

type DuckBuildpack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DuckBuildpackSpec        `json:"spec"`
	Status buildapi.BuildpackStatus `json:"status"`
}

func (b *DuckBuildpack) GetName() string {
	return b.Name
}

type DuckBuildpackSpec struct {
	ServiceAccountRef *v1.ObjectReference
}
