package v1alpha2

import (
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (bs *BuildStatus) Error(err error) {
	bs.Conditions = corev1alpha1.Conditions{
		{
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
			Type:               corev1alpha1.ConditionSucceeded,
			Status:             corev1.ConditionFalse,
			Message:            err.Error(),
		},
	}
}
