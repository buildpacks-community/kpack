package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kpackcore "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
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
	bs.Conditions = kpackcore.Conditions{
		{
			LastTransitionTime: kpackcore.VolatileTime{Inner: v1.Now()},
			Type:               kpackcore.ConditionReady,
			Status:             corev1.ConditionTrue,
		},
	}
}
