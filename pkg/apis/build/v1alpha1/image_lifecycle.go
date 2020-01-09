package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kpackcore "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

const (
	BuilderNotFound = "BuilderNotFound"
	BuilderNotReady = "BuilderNotReady"
)

func (im *Image) BuilderNotFound() kpackcore.Conditions {
	return kpackcore.Conditions{
		{
			Type:               kpackcore.ConditionReady,
			Status:             corev1.ConditionFalse,
			Reason:             BuilderNotFound,
			Message:            fmt.Sprintf("Unable to find builder %s.", im.Spec.Builder.Name),
			LastTransitionTime: kpackcore.VolatileTime{Inner: metav1.Now()},
		},
	}
}
