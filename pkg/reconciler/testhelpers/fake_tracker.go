package testhelpers

import (
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type FakeTracker map[string]map[types.NamespacedName]struct{}

func (f FakeTracker) Track(ref v1.ObjectMetaAccessor, obj types.NamespacedName) error {
	_, ok := f[key(ref)]
	if !ok {
		f[key(ref)] = map[types.NamespacedName]struct{}{}
	}

	f[key(ref)][obj] = struct{}{}
	return nil
}

func (FakeTracker) OnChanged(obj interface{}) {
	panic("I should not be called in tests")
}

func (f FakeTracker) IsTracking(ref v1.ObjectMetaAccessor, obj types.NamespacedName) bool {
	trackingObs, ok := f[key(ref)]
	if !ok {
		return false
	}
	_, ok = trackingObs[obj]

	return ok
}

func key(ref v1.ObjectMetaAccessor) string {
	return fmt.Sprintf("%s/%s", ref.GetObjectMeta().GetNamespace(), ref.GetObjectMeta().GetName())
}
