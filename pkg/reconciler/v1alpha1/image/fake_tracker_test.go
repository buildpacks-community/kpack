package image_test

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

type fakeTracker map[corev1.ObjectReference]map[string]struct{}

func (f fakeTracker) Track(ref corev1.ObjectReference, obj interface{}) error {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}

	_, ok := f[ref]
	if !ok {
		f[ref] = map[string]struct{}{}
	}

	f[ref][key] = struct{}{}
	return nil
}

func (fakeTracker) OnChanged(obj interface{}) {
	panic("I should not be called in tests")
}

func (f fakeTracker) IsTracking(ref corev1.ObjectReference, obj interface{}) bool {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return false
	}

	trackingObs, ok := f[ref]
	if !ok {
		return false
	}
	_, ok = trackingObs[key]

	return ok
}
