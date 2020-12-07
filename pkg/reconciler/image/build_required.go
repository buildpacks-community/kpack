package image

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildchange"
)

type buildRequiredResult struct {
	ConditionStatus corev1.ConditionStatus
	ReasonsStr      string
	ChangesStr      string
}

func newBuildRequiredResult(summary buildchange.ChangeSummary) buildRequiredResult {
	var result buildRequiredResult
	if summary.HasChanges {
		result.ConditionStatus = corev1.ConditionTrue
	} else {
		result.ConditionStatus = corev1.ConditionFalse
	}
	result.ReasonsStr = summary.ReasonsStr
	result.ChangesStr = summary.ChangesStr
	return result
}

func isBuildRequired(img *v1alpha1.Image,
	lastBuild *v1alpha1.Build,
	srcResolver *v1alpha1.SourceResolver,
	builder v1alpha1.BuilderResource) (buildRequiredResult, error) {

	result := buildRequiredResult{ConditionStatus: corev1.ConditionUnknown}
	if !srcResolver.Ready() || !builder.Ready() {
		return result, nil
	}

	changeSummary, err := buildchange.NewChangeProcessor().
		Process(triggerChange(lastBuild)).
		Process(commitChange(lastBuild, srcResolver)).
		Process(configChange(img, lastBuild, srcResolver)).
		Process(buildpackChange(lastBuild, builder)).
		Process(stackChange(lastBuild, builder)).
		Summarize()
	if err != nil {
		return result, err
	}

	return newBuildRequiredResult(changeSummary), nil
}

func triggerChange(lastBuild *v1alpha1.Build) buildchange.Change {
	if lastBuild == nil || lastBuild.Annotations == nil {
		return nil
	}

	time, ok := lastBuild.Annotations[v1alpha1.BuildNeededAnnotation]
	if !ok {
		return nil
	}
	return buildchange.NewTriggerChange(time)
}

func commitChange(lastBuild *v1alpha1.Build, srcResolver *v1alpha1.SourceResolver) buildchange.Change {
	// If the lastBuild was not a Git source, then it is not a COMMIT change
	if lastBuild == nil || lastBuild.Spec.Source.Git == nil || srcResolver.Status.Source.Git == nil {
		return nil
	}

	oldRevision := lastBuild.Spec.Source.Git.Revision
	newRevision := srcResolver.Status.Source.Git.Revision
	return buildchange.NewCommitChange(oldRevision, newRevision)
}

func configChange(img *v1alpha1.Image, lastBuild *v1alpha1.Build, srcResolver *v1alpha1.SourceResolver) buildchange.Change {
	var old buildchange.Config
	var new buildchange.Config

	if lastBuild != nil {
		old = buildchange.Config{
			Env:       lastBuild.Spec.Env,
			Resources: lastBuild.Spec.Resources,
			Bindings:  lastBuild.Spec.Bindings,
			Source:    lastBuild.Spec.Source,
		}
	}

	new = buildchange.Config{
		Env:       img.Env(),
		Resources: img.Resources(),
		Bindings:  img.Bindings(),
		Source:    srcResolver.Status.Source.ResolvedSource().SourceConfig(),
	}

	return buildchange.NewConfigChange(old, new)
}

func buildpackChange(lastBuild *v1alpha1.Build, builder v1alpha1.BuilderResource) buildchange.Change {
	if lastBuild == nil || !lastBuild.IsSuccess() {
		return nil
	}

	builderBuildpacks := builder.BuildpackMetadata()
	getBuilderBuildpackById := func(bpId string) *v1alpha1.BuildpackMetadata {
		for _, bp := range builderBuildpacks {
			if bp.Id == bpId {
				return &bp
			}
		}
		return nil
	}

	var old []v1alpha1.BuildpackInfo
	var new []v1alpha1.BuildpackInfo

	for _, lastBuildBp := range lastBuild.Status.BuildMetadata {
		builderBp := getBuilderBuildpackById(lastBuildBp.Id)
		if builderBp == nil {
			old = append(old, v1alpha1.BuildpackInfo{Id: lastBuildBp.Id, Version: lastBuildBp.Version})
		} else if builderBp.Version != lastBuildBp.Version {
			old = append(old, v1alpha1.BuildpackInfo{Id: lastBuildBp.Id, Version: lastBuildBp.Version})
			new = append(new, v1alpha1.BuildpackInfo{Id: builderBp.Id, Version: builderBp.Version})
		}
	}

	return buildchange.NewBuildpackChange(old, new)
}

func stackChange(lastBuild *v1alpha1.Build, builder v1alpha1.BuilderResource) buildchange.Change {
	if lastBuild == nil || !lastBuild.IsSuccess() {
		return nil
	}

	oldRunImageRefStr := lastBuild.Status.Stack.RunImage
	newRunImageRefStr := builder.RunImage()
	return buildchange.NewStackChange(oldRunImageRefStr, newRunImageRefStr)
}
