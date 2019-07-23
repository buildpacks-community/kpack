package v1alpha1

import "strconv"

type BuildCreator interface {
	CreateBuild(*Build) (*Build, error)
}

type ReconciledBuild struct {
	Build        *Build
	BuildCounter int64
	LatestImage  string
}

type BuildApplier interface {
	Apply(creator BuildCreator) (ReconciledBuild, error)
}

type upToDateBuild struct {
	build        *Build
	buildCounter int64
	latestImage  string
}

func (r upToDateBuild) Apply(creator BuildCreator) (ReconciledBuild, error) {
	return ReconciledBuild{
		Build:        r.build,
		BuildCounter: r.buildCounter,
		LatestImage:  r.latestImage,
	}, nil
}

type newBuild struct {
	build        *Build
	buildCounter int64
	latestImage  string
}

func (r newBuild) Apply(creator BuildCreator) (ReconciledBuild, error) {
	build, err := creator.CreateBuild(r.build)
	return ReconciledBuild{
		Build:        build,
		BuildCounter: r.buildCounter,
		LatestImage:  r.latestImage,
	}, err
}

func (im *Image) ReconcileBuild(latestBuild *Build, resolver *SourceResolver, builder *Builder) (BuildApplier, error) {
	currentBuildNumber, err := buildCounter(latestBuild)
	if err != nil {
		return nil, err
	}
	latestImage := im.latestForImage(latestBuild)

	if reasons, needed := im.buildNeeded(latestBuild, resolver, builder); needed {
		nextBuildNumber := currentBuildNumber + 1
		return newBuild{
			build:        im.build(resolver, builder, reasons, nextBuildNumber),
			buildCounter: nextBuildNumber,
			latestImage:  latestImage,
		}, nil
	}

	return upToDateBuild{build: latestBuild, buildCounter: currentBuildNumber, latestImage: latestImage}, nil
}

func buildCounter(build *Build) (int64, error) {
	if build == nil {
		return 0, nil
	}

	buildNumber := build.Labels[BuildNumberLabel]
	return strconv.ParseInt(buildNumber, 10, 64)
}
