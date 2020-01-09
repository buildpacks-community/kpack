package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	kpackcore "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func (cb *CustomBuilderStatus) ErrorCreate(err error) {
	cb.BuilderStatus = v1alpha1.BuilderStatus{
		Status: kpackcore.Status{
			Conditions: kpackcore.Conditions{
				{
					Type:               kpackcore.ConditionReady,
					Status:             corev1.ConditionFalse,
					LastTransitionTime: kpackcore.VolatileTime{Inner: v1.Now()},
					Message:            err.Error(),
				},
			},
		},
	}

}
