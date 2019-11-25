package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

func (cb *CustomBuilder) ErrorCreate(err error) {
	cb.Status = v1alpha1.BuilderStatus{
		Status: duckv1alpha1.Status{
			Conditions: duckv1alpha1.Conditions{
				{
					Type:               duckv1alpha1.ConditionReady,
					Status:             corev1.ConditionFalse,
					LastTransitionTime: apis.VolatileTime{Inner: v1.Now()},
					Message:            err.Error(),
				},
			},
		},
	}

}
