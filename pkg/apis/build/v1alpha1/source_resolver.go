package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const ActivePolling = "ActivePolling"

func (sr *SourceResolver) ResolvedSource(resolvedSourceInterface ResolvedSource) {
	if resolvedSourceInterface.IsUnknown() && sr.Status.ObservedGeneration == sr.ObjectMeta.Generation {
		return
	}

	switch resolvedSource := resolvedSourceInterface.(type) {
	case *ResolvedGitSource:
		sr.Status.Source.Git = resolvedSource
	case *ResolvedBlobSource:
		sr.Status.Source.Blob = resolvedSource
	case *ResolvedRegistrySource:
		sr.Status.Source.Registry = resolvedSource
	}

	sr.Status.Conditions = []duckv1alpha1.Condition{{
		Type:   duckv1alpha1.ConditionReady,
		Status: corev1.ConditionTrue,
	}}

	pollingStatus := corev1.ConditionFalse
	if resolvedSourceInterface.IsPollable() {
		pollingStatus = corev1.ConditionTrue
	}
	sr.Status.Conditions = append(sr.Status.Conditions, duckv1alpha1.Condition{
		Type:   ActivePolling,
		Status: pollingStatus,
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
