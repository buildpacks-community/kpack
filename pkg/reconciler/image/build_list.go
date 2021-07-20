package image

import (
	"sort"

	v1alpha1build "github.com/pivotal/kpack/pkg/reconciler/build"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
)

type buildList struct {
	successfulBuilds []*buildapi.Build
	failedBuilds     []*buildapi.Build
	lastBuild        *buildapi.Build
}

func newBuildList(builds []*buildapi.Build) (buildList, error) {
	sort.Sort(v1alpha1build.ByCreationTimestamp(builds)) //nobody enforcing this

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

func (l buildList) OldestFailure() *buildapi.Build {
	return l.failedBuilds[0]
}

func (l buildList) NumberSuccessfulBuilds() int64 {
	return int64(len(l.successfulBuilds))
}

func (l buildList) OldestSuccess() *buildapi.Build {
	return l.successfulBuilds[0]
}
