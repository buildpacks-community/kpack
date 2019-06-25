package image

import (
	"github.com/knative/pkg/kmeta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
)

func MakeBuildCache(image *v1alpha1.Image) (*corev1.PersistentVolumeClaim, error) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: image.CacheName(),
			Namespace:    image.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(image),
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *image.Spec.CacheSize,
				},
			},
		},
	}

	return pvc, nil
}
