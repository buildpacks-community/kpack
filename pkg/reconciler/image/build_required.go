package image

import (
	"time"

	corev1 "k8s.io/api/core/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/buildchange"
)

type buildRequiredResult struct {
	ConditionStatus corev1.ConditionStatus
	ReasonsStr      string
	ChangesStr      string
	PriorityClass   string
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
	result.PriorityClass = summary.Priority.PriorityClass()
	return result
}

func isBuildRequired(img *buildapi.Image,
	lastBuild *buildapi.Build,
	srcResolver *buildapi.SourceResolver,
	builder buildapi.BuilderResource) (buildRequiredResult, error) {

	result := buildRequiredResult{ConditionStatus: corev1.ConditionUnknown}
	if !srcResolver.Ready() || !builder.Ready() {
		return result, nil
	}

	changeSummary, err := buildchange.NewChangeProcessor().
		Process(triggerChange(lastBuild)).
		Process(commitChange(lastBuild, srcResolver)).
		Process(configChange(img, lastBuild, srcResolver)).
		Process(buildpackChange(lastBuild, builder)).
		Process(extensionChange(lastBuild, builder)).
		Process(stackChange(lastBuild, builder)).
		Summarize()
	if err != nil {
		return result, err
	}

	return newBuildRequiredResult(changeSummary), nil
}

func triggerChange(lastBuild *buildapi.Build) buildchange.Change {
	if lastBuild == nil || lastBuild.Annotations == nil {
		return nil
	}

	_, ok := lastBuild.Annotations[buildapi.BuildNeededAnnotation]
	if !ok {
		return nil
	}

	time := time.Now().Format(time.RFC1123Z)
	return buildchange.NewTriggerChange(time)
}

func commitChange(lastBuild *buildapi.Build, srcResolver *buildapi.SourceResolver) buildchange.Change {
	// If the lastBuild was not a Git source, then it is not a COMMIT change
	if lastBuild == nil || lastBuild.Spec.Source.Git == nil || srcResolver.Status.Source.Git == nil {
		return nil
	}

	oldRevision := lastBuild.Spec.Source.Git.Revision
	newRevision := srcResolver.Status.Source.Git.Revision
	return buildchange.NewCommitChange(oldRevision, newRevision)
}

func configChange(img *buildapi.Image, lastBuild *buildapi.Build, srcResolver *buildapi.SourceResolver) buildchange.Change {
	var old buildchange.Config
	var new buildchange.Config

	if lastBuild != nil {
		old = buildchange.Config{
			Env:         lastBuild.Spec.Env,
			Resources:   lastBuild.Spec.Resources,
			Services:    lastBuild.Spec.Services,
			CNBBindings: lastBuild.Spec.CNBBindings,
			Source:      lastBuild.Spec.Source,
		}
	}

	new = buildchange.Config{
		Env:         img.Env(),
		Resources:   img.Resources(),
		Services:    img.Services(),
		CNBBindings: img.CNBBindings(),
		Source:      srcResolver.Status.Source.ResolvedSource().SourceConfig(),
	}

	return buildchange.NewConfigChange(old, new)
}

func buildpackChange(lastBuild *buildapi.Build, builder buildapi.BuilderResource) buildchange.Change {
	if lastBuild == nil || !lastBuild.IsSuccess() {
		return nil
	}

	var oldInfo []corev1alpha1.BuildpackInfo
	var newInfo []corev1alpha1.BuildpackInfo

	fromBuilder := builder.BuildpackMetadata()
	for _, fromLastBuild := range lastBuild.Status.BuildMetadataBuildpacks {
		if !fromBuilder.Include(fromLastBuild) {
			oldInfo = append(oldInfo, corev1alpha1.BuildpackInfo{Id: fromLastBuild.Id, Version: fromLastBuild.Version})
		}
	}

	return buildchange.NewBuildpackChange(oldInfo, newInfo)
}

func extensionChange(lastBuild *buildapi.Build, builder buildapi.BuilderResource) buildchange.Change {
	if lastBuild == nil || !lastBuild.IsSuccess() {
		return nil
	}

	var oldInfo []corev1alpha1.BuildpackInfo
	var newInfo []corev1alpha1.BuildpackInfo

	fromBuilder := builder.ExtensionMetadata()
	for _, fromLastBuild := range lastBuild.Status.BuildMetadataExtensions {
		if !fromBuilder.Include(fromLastBuild) {
			oldInfo = append(oldInfo, corev1alpha1.BuildpackInfo{Id: fromLastBuild.Id, Version: fromLastBuild.Version})
		}
	}

	return buildchange.NewExtensionChange(oldInfo, newInfo)
}

func stackChange(lastBuild *buildapi.Build, builder buildapi.BuilderResource) buildchange.Change {
	if lastBuild == nil || !lastBuild.IsSuccess() {
		return nil
	}

	if len(builder.ExtensionMetadata()) > 0 {
		return nil
	}

	oldRunImageRefStr := lastBuild.Status.Stack.RunImage
	newRunImageRefStr := builder.RunImage()
	return buildchange.NewStackChange(oldRunImageRefStr, newRunImageRefStr)
}
