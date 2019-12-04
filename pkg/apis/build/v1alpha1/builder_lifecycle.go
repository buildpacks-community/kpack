package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

type BuilderRecord struct {
	Image      string
	Stack      BuildStack
	Buildpacks BuildpackMetadataList
}

func (bs *BuilderStatus) BuilderRecord(record BuilderRecord) {
	bs.Stack = record.Stack
	bs.BuilderMetadata = record.Buildpacks
	bs.LatestImage = record.Image
	bs.Conditions = duckv1alpha1.Conditions{
		{
			LastTransitionTime: apis.VolatileTime{Inner: v1.Now()},
			Type:               duckv1alpha1.ConditionReady,
			Status:             corev1.ConditionTrue,
		},
	}
}
