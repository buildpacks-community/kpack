package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

const ActivePolling = "ActivePolling"

func (sr *SourceResolver) ResolvedSource(resolvedSource ResolvedSource) {
	if resolvedSource.Git != nil {
		sr.ResolvedGitSource(resolvedSource.Git)
	} else if resolvedSource.Blob != nil {
		sr.ResolvedBlobSource(resolvedSource.Blob)
	} else if resolvedSource.Registry != nil {
		sr.ResolvedRegistrySource(resolvedSource.Registry)
	}
}

func (sr *SourceResolver) ResolvedGitSource(resolvedGitSource *ResolvedGitSource) {
	if resolvedGitSource.IsUnknown() && sr.Status.ObservedGeneration == sr.ObjectMeta.Generation {
		return
	}

	sr.Status.ResolvedSource.Git = resolvedGitSource

	sr.Status.Conditions = []duckv1alpha1.Condition{{
		Type:   duckv1alpha1.ConditionReady,
		Status: corev1.ConditionTrue,
	}}

	if resolvedGitSource.IsPollable() {
		sr.Status.Conditions = append(sr.Status.Conditions, duckv1alpha1.Condition{
			Type:   ActivePolling,
			Status: corev1.ConditionTrue,
		})
	} else {
		sr.Status.Conditions = append(sr.Status.Conditions, duckv1alpha1.Condition{
			Type:   ActivePolling,
			Status: corev1.ConditionFalse,
		})
	}
}

func (sr *SourceResolver) ResolvedBlobSource(resolvedBlobSource *ResolvedBlobSource) {
	sr.Status.ResolvedSource.Blob = resolvedBlobSource

	sr.Status.Conditions = []duckv1alpha1.Condition{{
		Type:   duckv1alpha1.ConditionReady,
		Status: corev1.ConditionTrue,
	}}

	sr.Status.Conditions = append(sr.Status.Conditions, duckv1alpha1.Condition{
		Type:   ActivePolling,
		Status: corev1.ConditionFalse,
	})
}

func (sr *SourceResolver) ResolvedRegistrySource(resolvedRegistrySource *ResolvedRegistrySource) {
	sr.Status.ResolvedSource.Registry = resolvedRegistrySource

	sr.Status.Conditions = []duckv1alpha1.Condition{{
		Type:   duckv1alpha1.ConditionReady,
		Status: corev1.ConditionTrue,
	}}

	sr.Status.Conditions = append(sr.Status.Conditions, duckv1alpha1.Condition{
		Type:   ActivePolling,
		Status: corev1.ConditionFalse,
	})
}

func (sr *SourceResolver) PollingReady() bool {
	return sr.Status.GetCondition(ActivePolling).IsTrue()
}

func (sr *SourceResolver) Ready() bool {
	return sr.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue() &&
		(sr.Generation == sr.Status.ObservedGeneration)
}

func (sr SourceResolver) IsGit() bool {
	return sr.Spec.Source.Git != nil
}

func (sr SourceResolver) IsBlob() bool {
	return sr.Spec.Source.Blob != nil
}

func (sr SourceResolver) IsRegistry() bool {
	return sr.Spec.Source.Registry != nil
}

func (sr SourceResolver) ConfigChanged(lastBuild *Build) bool {
	return sr.gitURLChanged(lastBuild) || sr.blobChanged(lastBuild) || sr.registryChanged(lastBuild)
}

func (sr SourceResolver) gitURLChanged(lastBuild *Build) bool {
	return sr.Status.ResolvedSource.Git != nil && sr.Status.ResolvedSource.Git.URL != lastBuild.Spec.Source.Git.URL
}

func (sr SourceResolver) GitRevisionChanged(lastBuild *Build) bool {
	return sr.Status.ResolvedSource.Git != nil && sr.Status.ResolvedSource.Git.Revision != lastBuild.Spec.Source.Git.Revision
}

func (sr SourceResolver) blobChanged(lastBuild *Build) bool {
	return sr.Status.ResolvedSource.Blob != nil && sr.Status.ResolvedSource.Blob.URL != lastBuild.Spec.Source.Blob.URL
}

func (sr SourceResolver) registryChanged(lastBuild *Build) bool {
	return sr.Status.ResolvedSource.Registry != nil &&
		(sr.Status.ResolvedSource.Registry.Image != lastBuild.Spec.Source.Registry.Image ||
			!equality.Semantic.DeepEqual(sr.Status.ResolvedSource.Registry.ImagePullSecrets, lastBuild.Spec.Source.Registry.ImagePullSecrets))
}

func (sr SourceResolver) desiredSource() Source {
	if sr.Status.ResolvedSource.Git != nil {
		return Source{
			Git: &Git{
				URL:      sr.Status.ResolvedSource.Git.URL,
				Revision: sr.Status.ResolvedSource.Git.Revision,
			},
		}
	} else if sr.Status.ResolvedSource.Blob != nil {
		return Source{
			Blob: &Blob{
				URL: sr.Status.ResolvedSource.Blob.URL,
			},
		}
	} else {
		return Source{
			Registry: &Registry{
				Image:            sr.Status.ResolvedSource.Registry.Image,
				ImagePullSecrets: sr.Status.ResolvedSource.Registry.ImagePullSecrets,
			},
		}
	}
}
