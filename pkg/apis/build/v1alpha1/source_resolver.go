package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

const ActivePolling = "ActivePolling"

func (sr *SourceResolver) ResolvedSource(config ResolvedSourceConfig) {
	resolvedSource := config.ResolvedSource()

	if resolvedSource.IsUnknown() && sr.Status.ObservedGeneration == sr.ObjectMeta.Generation {
		return
	}

	sr.Status.Source = config

	sr.Status.Conditions = []duckv1alpha1.Condition{{
		Type:   duckv1alpha1.ConditionReady,
		Status: corev1.ConditionTrue,
	}}

	pollingStatus := corev1.ConditionFalse
	if resolvedSource.IsPollable() {
		pollingStatus = corev1.ConditionTrue
	}
	sr.Status.Conditions = append(sr.Status.Conditions, duckv1alpha1.Condition{
		Type:   ActivePolling,
		Status: pollingStatus,
	})
}

func (sr *SourceResolver) ConfigChanged(lastBuild *Build) bool {
	return sr.Status.Source.ResolvedSource().ConfigChanged(lastBuild)
}

func (sr *SourceResolver) RevisionChanged(lastBuild *Build) bool {
	return sr.Status.Source.ResolvedSource().RevisionChanged(lastBuild)
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

func (st *SourceResolver) SourceConfig() SourceConfig {
	return st.Status.Source.ResolvedSource().SourceConfig()
}
