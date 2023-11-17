package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

type BuilderRecord struct {
	Image                   string
	Stack                   corev1alpha1.BuildStack
	Buildpacks              corev1alpha1.BuildpackMetadataList
	Order                   []corev1alpha1.OrderEntry
	ObservedStoreGeneration int64
	ObservedStackGeneration int64
	OS                      string
	SignaturePaths          []CosignSignature
}

func (bs *BuilderStatus) BuilderRecord(record BuilderRecord) {
	bs.Stack = record.Stack
	bs.BuilderMetadata = record.Buildpacks
	bs.LatestImage = record.Image
	bs.Conditions = corev1alpha1.Conditions{
		{
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: v1.Now()},
			Type:               corev1alpha1.ConditionReady,
			Status:             corev1.ConditionTrue,
		},
		{
			Type:               ConditionUpToDate,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: corev1alpha1.VolatileTime{Inner: v1.Now()},
		},
	}
	bs.Order = record.Order
	bs.ObservedStoreGeneration = record.ObservedStoreGeneration
	bs.ObservedStackGeneration = record.ObservedStackGeneration
	bs.OS = record.OS
	bs.SignaturePaths = record.SignaturePaths
}

func (bs *BuilderStatus) ErrorCreate(err error) {

	readyCondition := corev1alpha1.Condition{
		LastTransitionTime: corev1alpha1.VolatileTime{Inner: v1.Now()},
		Type:               corev1alpha1.ConditionReady,
		Status:             corev1.ConditionTrue,
	}
	if bs.LatestImage == "" {
		readyCondition.Status = corev1.ConditionFalse
		readyCondition.Message = NoLatestImageMessage
		readyCondition.Reason = NoLatestImageReason
	}

	bs.Status = corev1alpha1.Status{
		Conditions: corev1alpha1.Conditions{
			readyCondition,
			{
				Type:               ConditionUpToDate,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: corev1alpha1.VolatileTime{Inner: v1.Now()},
				Reason:             ReconcileFailedReason,
				Message:            err.Error(),
			},
		},
	}
}
