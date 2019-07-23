package v1alpha1

import "strconv"

type BuildCreator interface {
	CreateBuild(*Build) (*Build, error)
}

type ReconciledBuild struct {
	Build        *Build
	BuildCounter int64
	LastImage    string
}

type BuildApplier interface {
	Apply(creator BuildCreator) (ReconciledBuild, error)
}

type upToDateBuild struct {
	build        *Build
	buildCounter int64
	lastImage    string
}

func (r upToDateBuild) Apply(creator BuildCreator) (ReconciledBuild, error) {
	return ReconciledBuild{
		Build:        r.build,
		BuildCounter: r.buildCounter,
		LastImage:    r.lastImage,
	}, nil
}

type newBuild struct {
	build        *Build
	buildCounter int64
	lastImage    string
}

func (r newBuild) Apply(creator BuildCreator) (ReconciledBuild, error) {
	build, err := creator.CreateBuild(r.build)
	return ReconciledBuild{
		Build:        build,
		BuildCounter: r.buildCounter,
		LastImage:    r.lastImage,
	}, err
}

func (im *Image) ReconcileBuild(lastBuild *Build, resolver *SourceResolver, builder *Builder) (BuildApplier, error) {
	currentBuildNumber, err := buildCounter(lastBuild)
	if err != nil {
		return nil, err
	}
	lastImage := im.Status.LastImage
	if lastBuild.IsSuccess() {
		lastImage = lastBuild.BuiltImage()
	}

	if reasons, needed := im.buildNeeded(lastBuild, resolver, builder); needed {
		nextBuildNumber := currentBuildNumber + 1
		return newBuild{
			build:        im.build(resolver, builder, reasons, nextBuildNumber),
			buildCounter: nextBuildNumber,
			lastImage:    lastImage,
		}, nil
	}

	return upToDateBuild{build: lastBuild, buildCounter: currentBuildNumber, lastImage: lastImage}, nil
}

func buildCounter(build *Build) (int64, error) {
	if build == nil {
		return 0, nil
	}

	buildNumber := build.Labels[BuildNumberLabel]
	return strconv.ParseInt(buildNumber, 10, 64)
}
