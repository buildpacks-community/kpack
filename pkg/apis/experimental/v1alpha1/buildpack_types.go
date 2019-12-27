package v1alpha1

import "fmt"

type Order []OrderEntry

type OrderEntry struct {
	Group []BuildpackRef `json:"group"`
}

type BuildpackRef struct {
	BuildpackInfo
	Optional bool `json:"optional,omitempty"`
}

type BuildpackInfo struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}

func (b BuildpackInfo) String() string {
	return fmt.Sprintf("%s@%s", b.ID, b.Version)
}
