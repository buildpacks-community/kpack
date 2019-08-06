package v1alpha1

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
)

func (b *Builder) Ready() bool {
	return b.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue()
}
