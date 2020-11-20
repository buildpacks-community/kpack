package image

import (
	"sort"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	v1alpha2build "github.com/pivotal/kpack/pkg/reconciler/build"
)

type buildList struct {
	successfulBuilds []*v1alpha2.Build
	failedBuilds     []*v1alpha2.Build
	lastBuild        *v1alpha2.Build
}

func newBuildList(builds []*v1alpha2.Build) (buildList, error) {
	sort.Sort(v1alpha2build.ByCreationTimestamp(builds)) //nobody enforcing this

	buildList := buildList{}

	for _, build := range builds {
		if build.IsSuccess() {
			buildList.successfulBuilds = append(buildList.successfulBuilds, build)
		} else if build.IsFailure() {
			buildList.failedBuilds = append(buildList.failedBuilds, build)
		}
	}

	if len(builds) > 0 {
		buildList.lastBuild = builds[len(builds)-1]
	}

	return buildList, nil
}

func (l buildList) NumberFailedBuilds() int64 {
	return int64(len(l.failedBuilds))
}

func (l buildList) OldestFailure() *v1alpha2.Build {
	return l.failedBuilds[0]
}

func (l buildList) NumberSuccessfulBuilds() int64 {
	return int64(len(l.successfulBuilds))
}

func (l buildList) OldestSuccess() *v1alpha2.Build {
	return l.successfulBuilds[0]
}
