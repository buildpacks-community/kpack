package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const ActivePolling = "ActivePolling"

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

func (sr SourceResolver) GitURLChanged(lastBuild *Build) bool {
	return sr.Status.ResolvedSource.Git != nil && sr.Status.ResolvedSource.Git.URL != lastBuild.Spec.Source.Git.URL
}

func (sr SourceResolver) GitRevisionChanged(lastBuild *Build) bool {
	return sr.Status.ResolvedSource.Git != nil && sr.Status.ResolvedSource.Git.Revision != lastBuild.Spec.Source.Git.Revision
}

func (sr SourceResolver) BlobChanged(lastBuild *Build) bool {
	return sr.Status.ResolvedSource.Blob != nil && sr.Status.ResolvedSource.Blob.URL != lastBuild.Spec.Source.Blob.URL
}

func (sr SourceResolver) desiredSource() Source {
	if sr.Status.ResolvedSource.Git != nil {
		return Source{
			Git: &Git{
				URL:      sr.Status.ResolvedSource.Git.URL,
				Revision: sr.Status.ResolvedSource.Git.Revision,
			},
		}
	} else {
		return Source{
			Blob: &Blob{
				URL: sr.Status.ResolvedSource.Blob.URL,
			},
		}
	}
}
