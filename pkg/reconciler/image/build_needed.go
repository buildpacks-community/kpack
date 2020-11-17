package image

import (
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildchange"
)

type BuildDeterminer struct {
	img         *v1alpha1.Image
	lastBuild   *v1alpha1.Build
	srcResolver *v1alpha1.SourceResolver
	builder     v1alpha1.BuilderResource

	lastBuildRunImageRef name.Reference
	builderRunImageRef   name.Reference

	changeSummary buildchange.ChangeSummary
}

func NewBuildDeterminer(
	img *v1alpha1.Image,
	lastBuild *v1alpha1.Build,
	srcResolver *v1alpha1.SourceResolver,
	builder v1alpha1.BuilderResource) *BuildDeterminer {

	return &BuildDeterminer{
		img:         img,
		lastBuild:   lastBuild,
		srcResolver: srcResolver,
		builder:     builder,
	}
}

func (b *BuildDeterminer) IsBuildNeeded() (corev1.ConditionStatus, error) {
	if !b.readyToDetermine() {
		return corev1.ConditionUnknown, nil
	}

	err := b.validateState()
	if err != nil {
		return corev1.ConditionUnknown, err
	}

	b.changeSummary, err = buildchange.NewChangeProcessor().
		Process(b.triggerChange()).
		Process(b.commitChange()).
		Process(b.configChange()).
		Process(b.buildpackChange()).
		Process(b.stackChange()).Summarize()
	if err != nil {
		return corev1.ConditionUnknown, err
	}

	if b.changeSummary.HasChanges {
		return corev1.ConditionTrue, nil
	}
	return corev1.ConditionFalse, nil
}

func (b *BuildDeterminer) Reasons() string {
	return b.changeSummary.ReasonsStr
}

func (b *BuildDeterminer) Changes() string {
	return b.changeSummary.ChangesStr
}

func (b *BuildDeterminer) readyToDetermine() bool {
	return b.srcResolver.Ready() && b.builder.Ready()
}

func (b *BuildDeterminer) validateState() error {
	var err error
	if !b.lastBuildWasSuccessful() {
		return err
	}

	b.lastBuildRunImageRef, err = name.ParseReference(b.lastBuild.Status.Stack.RunImage)
	if err != nil {
		return errors.Wrapf(err, "cannot parse last build run image reference '%s'", b.lastBuild.Status.Stack.RunImage)
	}

	b.builderRunImageRef, err = name.ParseReference(b.builder.RunImage())
	if err != nil {
		return errors.Wrapf(err, "cannot parse builder run image reference '%s'", b.builder.RunImage())
	}
	return err
}

func (b *BuildDeterminer) triggerChange() buildchange.TriggerChange {
	var change buildchange.TriggerChange
	if b.lastBuild == nil || b.lastBuild.Annotations == nil {
		return change
	}

	time, ok := b.lastBuild.Annotations[v1alpha1.BuildNeededAnnotation]
	if ok {
		change = buildchange.TriggerChange{New: time}
	}
	return change
}

func (b *BuildDeterminer) commitChange() buildchange.CommitChange {
	if b.lastBuild == nil || b.srcResolver.Status.Source.Git == nil {
		return buildchange.CommitChange{}
	}

	return buildchange.CommitChange{
		Old: b.lastBuild.Spec.Source.Git.Revision,
		New: b.srcResolver.Status.Source.Git.Revision,
	}
}

func (b *BuildDeterminer) configChange() buildchange.ConfigChange {
	var old buildchange.Config
	if b.lastBuild != nil {
		old = buildchange.Config{
			Env:       b.lastBuild.Spec.Env,
			Resources: b.lastBuild.Spec.Resources,
			Bindings:  b.lastBuild.Spec.Bindings,
			Source:    b.lastBuild.Spec.Source,
		}
	}

	return buildchange.ConfigChange{
		New: buildchange.Config{
			Env:       b.img.Env(),
			Resources: b.img.Resources(),
			Bindings:  b.img.Bindings(),
			Source:    b.srcResolver.Status.Source.ResolvedSource().SourceConfig(),
		},
		Old: old,
	}
}

func (b *BuildDeterminer) buildpackChange() buildchange.BuildpackChange {
	if !b.lastBuildWasSuccessful() {
		return buildchange.BuildpackChange{}
	}

	builderBuildpacks := b.builder.BuildpackMetadata()
	getBuilderBuildpackById := func(bpId string) *v1alpha1.BuildpackMetadata {
		for _, bp := range builderBuildpacks {
			if bp.Id == bpId {
				return &bp
			}
		}
		return nil
	}

	var old []buildchange.BuildpackInfo
	var new []buildchange.BuildpackInfo

	for _, lastBuildBp := range b.lastBuild.Status.BuildMetadata {
		builderBp := getBuilderBuildpackById(lastBuildBp.Id)
		if builderBp == nil {
			old = append(old, buildchange.BuildpackInfo{Id: lastBuildBp.Id, Version: lastBuildBp.Version})
		} else if builderBp.Version != lastBuildBp.Version {
			old = append(old, buildchange.BuildpackInfo{Id: lastBuildBp.Id, Version: lastBuildBp.Version})
			new = append(new, buildchange.BuildpackInfo{Id: builderBp.Id, Version: builderBp.Version})
		}
	}
	return buildchange.BuildpackChange{Old: old, New: new}
}

func (b *BuildDeterminer) stackChange() buildchange.StackChange {
	var change buildchange.StackChange
	if b.lastBuildWasSuccessful() {
		change = buildchange.StackChange{
			Old: b.lastBuildRunImageRef.Identifier(),
			New: b.builderRunImageRef.Identifier(),
		}
	}
	return change
}

func (b *BuildDeterminer) lastBuildWasSuccessful() bool {
	return b.lastBuild != nil && b.lastBuild.IsSuccess()
}
