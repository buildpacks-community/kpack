package v1alpha2

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	BuilderNotFound    = "BuilderNotFound"
	BuilderNotReady    = "BuilderNotReady"
	BuilderReady       = "BuilderReady"
	BuilderNotUpToDate = "BuilderNotUpToDate"
	BuilderUpToDate    = "BuilderUpToDate"
)

func (im *Image) BuilderNotFound() corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:               corev1alpha1.ConditionReady,
			Status:             corev1.ConditionFalse,
			Reason:             BuilderNotFound,
			Message:            fmt.Sprintf("Error: Unable to find builder '%s' in namespace '%s'.", im.Spec.Builder.Name, im.Spec.Builder.Namespace),
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: metav1.Now()},
		},
	}
}
