package buildchange

import (
	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

func NewLifecycleChange(oldLifecycle, newLifecycle string) Change {
	return lifecycleChange{
		oldLifecycle: oldLifecycle,
		newLifecycle: newLifecycle,
	}
}

type lifecycleChange struct {
	oldLifecycle string
	newLifecycle string
	err          error
}

func (l lifecycleChange) Reason() buildapi.BuildReason { return buildapi.BuildReasonLifecycle }

func (l lifecycleChange) IsBuildRequired() (bool, error) {
	return l.oldLifecycle != l.newLifecycle, l.err
}

func (l lifecycleChange) Old() interface{} { return l.oldLifecycle }

func (l lifecycleChange) New() interface{} { return l.newLifecycle }

func (l lifecycleChange) Priority() buildapi.BuildPriority { return buildapi.BuildPriorityLow }
