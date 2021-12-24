package buildchange

import (
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

func NewStackChange(oldRunImageRefStr, newRunImageRefStr string) Change {
	var change stackChange
	var errStrs []string

	oldRunImageRef, err := name.ParseReference(oldRunImageRefStr)
	if err != nil {
		errStrs = append(errStrs, err.Error())
	} else {
		change.oldRunImageDigest = oldRunImageRef.Identifier()
	}

	newRunImageRef, err := name.ParseReference(newRunImageRefStr)
	if err != nil {
		errStrs = append(errStrs, err.Error())
	} else {
		change.newRunImageDigest = newRunImageRef.Identifier()
	}

	if len(errStrs) > 0 {
		change.err = errors.New(strings.Join(errStrs, "; "))
	}
	return change
}

type stackChange struct {
	oldRunImageDigest string
	newRunImageDigest string
	err               error
}

func (s stackChange) Reason() buildapi.BuildReason { return buildapi.BuildReasonStack }

func (s stackChange) IsBuildRequired() (bool, error) {
	return s.oldRunImageDigest != s.newRunImageDigest, s.err
}

func (s stackChange) Old() interface{} { return s.oldRunImageDigest }

func (s stackChange) New() interface{} { return s.newRunImageDigest }

func (s stackChange) Priority() buildapi.BuildPriority { return buildapi.BuildPriorityLow }
