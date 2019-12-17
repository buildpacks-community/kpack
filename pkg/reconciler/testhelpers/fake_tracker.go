package testhelpers

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type FakeTracker map[types.UID]map[types.NamespacedName]struct{}

func (f FakeTracker) Track(ref v1.ObjectMetaAccessor, obj types.NamespacedName) error {
	key := ref.GetObjectMeta().GetUID()

	_, ok := f[key]
	if !ok {
		f[key] = map[types.NamespacedName]struct{}{}
	}

	f[key][obj] = struct{}{}
	return nil
}

func (FakeTracker) OnChanged(obj interface{}) {
	panic("I should not be called in tests")
}

func (f FakeTracker) IsTracking(ref v1.ObjectMetaAccessor, obj types.NamespacedName) bool {
	trackingObs, ok := f[ref.GetObjectMeta().GetUID()]
	if !ok {
		return false
	}
	_, ok = trackingObs[obj]

	return ok
}
