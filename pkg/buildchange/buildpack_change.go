package buildchange

import (
	"sort"

	"github.com/google/go-cmp/cmp"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func NewBuildpackChange(oldBuildpacks, newBuildpacks []buildapi.BuildpackInfo) Change {
	return buildpackChange{
		old: oldBuildpacks,
		new: newBuildpacks,
	}
}

type buildpackChange struct {
	old []buildapi.BuildpackInfo
	new []buildapi.BuildpackInfo
}

func (b buildpackChange) Reason() buildapi.BuildReason { return buildapi.BuildReasonBuildpack }

func (b buildpackChange) IsBuildRequired() (bool, error) {
	sort.Slice(b.old, func(i, j int) bool {
		return b.old[i].Id < b.old[j].Id
	})
	sort.Slice(b.new, func(i, j int) bool {
		return b.new[i].Id < b.new[j].Id
	})
	return !cmp.Equal(b.old, b.new), nil
}

func (b buildpackChange) Old() interface{} { return b.old }

func (b buildpackChange) New() interface{} { return b.new }
