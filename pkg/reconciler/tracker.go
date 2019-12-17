package reconciler

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Tracker interface {
	Track(ref metav1.ObjectMetaAccessor, obj types.NamespacedName) error
	OnChanged(obj interface{})
}
