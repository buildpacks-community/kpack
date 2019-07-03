package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const ActivePolling = "ActivePolling"

func (sr *SourceResolver) ResolvedGitSource(resolvedGitSource ResolvedGitSource) {
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

func (sr *SourceResolver) PollingReady() bool {
	return sr.Status.GetCondition(ActivePolling).IsTrue()
}

func (sr *SourceResolver) Ready() bool {
	return sr.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue() &&
		(sr.Generation == sr.Status.ObservedGeneration)
}
