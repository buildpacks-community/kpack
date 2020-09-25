package v1alpha1

import "fmt"

type Order []OrderEntry

// +k8s:openapi-gen=true
type OrderEntry struct {
	// +listType
	Group []BuildpackRef `json:"group,omitempty" yaml:"group,omitempty"`
}

// +k8s:openapi-gen=true
type BuildpackRef struct {
	BuildpackInfo `json:",inline" yaml:",inline"`
	Optional      bool `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// +k8s:openapi-gen=true
type BuildpackInfo struct {
	Id      string `json:"id" yaml:"id"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

func (b BuildpackInfo) String() string {
	return fmt.Sprintf("%s@%s", b.Id, b.Version)
}

// +k8s:openapi-gen=true
type BuildpackStack struct {
	ID string `json:"id"`

	// +listType
	Mixins []string `json:"mixins,omitempty"`
}
