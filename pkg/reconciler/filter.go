package reconciler

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func FilterDeletionTimestamp(obj interface{}) bool {
	object, ok := obj.(metav1.Object)
	if !ok {
		return false
	}

	return object.GetDeletionTimestamp() == nil
}
