package buildchange

import (
	"sort"

	"github.com/google/go-cmp/cmp"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func NewBuildpackChange(oldBuildpacks, newBuildpacks []v1alpha1.BuildpackInfo) Change {
	return buildpackChange{
		old: oldBuildpacks,
		new: newBuildpacks,
	}
}

type buildpackChange struct {
	old []v1alpha1.BuildpackInfo
	new []v1alpha1.BuildpackInfo
}

func (b buildpackChange) Reason() v1alpha1.BuildReason { return v1alpha1.BuildReasonBuildpack }

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
