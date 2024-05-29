package buildchange

import (
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

func NewLifecycleChange(oldLifecycleImageRefStr, newLifecycleImageRefStr string) Change {
	var change lifecycleChange
	var errStrs []string

	oldLifecycleImageRef, err := name.ParseReference(oldLifecycleImageRefStr)
	if err != nil {
		errStrs = append(errStrs, err.Error())
	} else {
		change.oldLifecycleImageDigest = oldLifecycleImageRef.Identifier()
	}

	newLifecycleImageRef, err := name.ParseReference(newLifecycleImageRefStr)
	if err != nil {
		errStrs = append(errStrs, err.Error())
	} else {
		change.newLifecycleImageDigest = newLifecycleImageRef.Identifier()
	}

	if len(errStrs) > 0 {
		change.err = errors.New(strings.Join(errStrs, "; "))
	}
	return change
}

type lifecycleChange struct {
	oldLifecycleImageDigest string
	newLifecycleImageDigest string
	err                     error
}

func (l lifecycleChange) Reason() buildapi.BuildReason { return buildapi.BuildReasonLifecycle }

func (l lifecycleChange) IsBuildRequired() (bool, error) {
	return l.oldLifecycleImageDigest != l.newLifecycleImageDigest, l.err
}

func (l lifecycleChange) Old() interface{} { return l.oldLifecycleImageDigest }

func (l lifecycleChange) New() interface{} { return l.newLifecycleImageDigest }

func (l lifecycleChange) Priority() buildapi.BuildPriority { return buildapi.BuildPriorityLow }
