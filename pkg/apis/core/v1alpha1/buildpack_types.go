package v1alpha1

import "fmt"

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type StoreImage struct {
	Image string `json:"image,omitempty"`
}

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type StoreBuildpack struct {
	BuildpackInfo `json:",inline"`
	Buildpackage  BuildpackageInfo `json:"buildpackage,omitempty"`
	StoreImage    StoreImage       `json:"storeImage,omitempty"`
	DiffId        string           `json:"diffId,omitempty"`
	Digest        string           `json:"digest,omitempty"`
	Size          int64            `json:"size,omitempty"`
	API           string           `json:"api,omitempty"`
	Homepage      string           `json:"homepage,omitempty"`
	// +listType
	Order []OrderEntry `json:"order,omitempty"`
	// +listType
	Stacks []BuildpackStack `json:"stacks,omitempty"`
}

type Order []OrderEntry

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type OrderEntry struct {
	// +listType
	Group []BuildpackRef `json:"group,omitempty"`
}

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type BuildpackRef struct {
	BuildpackInfo `json:",inline"`
	Optional      bool `json:"optional,omitempty"`
}

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type BuildpackInfo struct {
	Id      string `json:"id"`
	Version string `json:"version,omitempty"`
}

func (b BuildpackInfo) String() string {
	return fmt.Sprintf("%s@%s", b.Id, b.Version)
}

// +k8s:openapi-gen=true
// +k8s:deepcopy-gen=true
type BuildpackStack struct {
	ID string `json:"id"`

	// +listType
	Mixins []string `json:"mixins,omitempty"`
}
