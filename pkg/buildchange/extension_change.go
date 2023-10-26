package buildchange

import (
	"sort"

	"github.com/google/go-cmp/cmp"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func NewExtensionChange(oldInfos, newInfos []corev1alpha1.BuildpackInfo) Change {
	return extensionChange{
		old: oldInfos,
		new: newInfos,
	}
}

type extensionChange struct {
	old []corev1alpha1.BuildpackInfo
	new []corev1alpha1.BuildpackInfo
}

func (b extensionChange) Reason() buildapi.BuildReason { return buildapi.BuildReasonExtension }

func (b extensionChange) IsBuildRequired() (bool, error) {
	sort.Slice(b.old, func(i, j int) bool {
		return b.old[i].Id < b.old[j].Id
	})
	sort.Slice(b.new, func(i, j int) bool {
		return b.new[i].Id < b.new[j].Id
	})
	return !cmp.Equal(b.old, b.new), nil
}

func (b extensionChange) Old() interface{} { return b.old }

func (b extensionChange) New() interface{} { return b.new }

func (b extensionChange) Priority() buildapi.BuildPriority { return buildapi.BuildPriorityLow }
