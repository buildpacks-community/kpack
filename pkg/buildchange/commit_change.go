package buildchange

import buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"

func NewCommitChange(oldRevision, newRevision string) Change {
	return commitChange{
		oldRevision: oldRevision,
		newRevision: newRevision,
	}
}

type commitChange struct {
	newRevision string
	oldRevision string
}

func (c commitChange) Reason() buildapi.BuildReason { return buildapi.BuildReasonCommit }

func (c commitChange) IsBuildRequired() (bool, error) { return c.oldRevision != c.newRevision, nil }

func (c commitChange) Old() interface{} { return c.oldRevision }

func (c commitChange) New() interface{} { return c.newRevision }

func (c commitChange) Priority() buildapi.BuildPriority { return buildapi.BuildPriorityHigh }
