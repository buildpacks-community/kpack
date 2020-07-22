package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func (cb *CustomBuilderStatus) ErrorCreate(err error) {
	cb.Status = corev1alpha1.Status{
		Conditions: corev1alpha1.Conditions{
			{
				Type:               corev1alpha1.ConditionReady,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: v1.Now()},
				Message:            err.Error(),
			},
		},
	}
}
