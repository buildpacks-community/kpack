package v1alpha1

import "strconv"

type BuildCreator interface {
	CreateBuild(*Build) (*Build, error)
}

type ReconciledBuild struct {
	Build        *Build
	BuildCounter int64
}

type BuildApplier interface {
	Apply(creator BuildCreator) (ReconciledBuild, error)
}

type upToDateBuild struct {
	build        *Build
	buildCounter int64
}

func (r upToDateBuild) Apply(creator BuildCreator) (ReconciledBuild, error) {
	return ReconciledBuild{
		Build:        r.build,
		BuildCounter: r.buildCounter,
	}, nil
}

type newBuild struct {
	build        *Build
	buildCounter int64
}

func (r newBuild) Apply(creator BuildCreator) (ReconciledBuild, error) {
	build, err := creator.CreateBuild(r.build)
	return ReconciledBuild{
		Build:        build,
		BuildCounter: r.buildCounter,
	}, err
}

func (im *Image) ReconcileBuild(lastBuild *Build, resolver *SourceResolver, builder *Builder) (BuildApplier, error) {
	currentBuildNumber, err := buildCounter(lastBuild)
	if err != nil {
		return nil, err
	}

	if reasons, needed := im.buildNeeded(lastBuild, resolver, builder); needed {
		nextBuildNumber := currentBuildNumber + 1
		return newBuild{
			build:        im.build(resolver, builder, reasons, nextBuildNumber),
			buildCounter: nextBuildNumber,
		}, nil
	}

	return upToDateBuild{build: lastBuild, buildCounter: currentBuildNumber}, nil
}

func buildCounter(build *Build) (int64, error) {
	if build == nil {
		return 0, nil
	}

	buildNumber := build.Labels[BuildNumberLabel]
	return strconv.ParseInt(buildNumber, 10, 64)
}
