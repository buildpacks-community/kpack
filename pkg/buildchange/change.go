package buildchange

import (
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

type Change interface {
	Reason() v1alpha1.BuildReason
	IsBuildRequired() (bool, error)
	Old() interface{}
	New() interface{}
}

type GenericChange struct {
	Reason string      `json:"reason,omitempty"`
	Old    interface{} `json:"old,omitempty"`
	New    interface{} `json:"new,omitempty"`
}

func newGenericChange(change Change) GenericChange {
	return GenericChange{
		Reason: string(change.Reason()),
		Old:    change.Old(),
		New:    change.New(),
	}
}
