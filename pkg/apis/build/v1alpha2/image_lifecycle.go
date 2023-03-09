package v1alpha2

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	BuilderNotFound = "BuilderNotFound"
	BuilderNotReady = "BuilderNotReady"
)

func (im *Image) BuilderNotFound() corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             corev1.ConditionFalse,
			Reason:             BuilderNotFound,
			Message:            fmt.Sprintf("Unable to find builder %s.", im.Spec.Builder.Name),
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
	}
}
