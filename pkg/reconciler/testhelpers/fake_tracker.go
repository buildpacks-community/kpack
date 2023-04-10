package testhelpers

import (
	"fmt"

	"github.com/pivotal/kpack/pkg/reconciler"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type FakeTracker struct {
	objects map[string]map[types.NamespacedName]struct{}
	kinds   map[string]map[types.NamespacedName]struct{}
}

func (f *FakeTracker) Track(ref reconciler.Key, obj types.NamespacedName) {
	if f.objects == nil {
		f.objects = make(map[string]map[types.NamespacedName]struct{})
	}

	_, ok := f.objects[ref.String()]
	if !ok {
		f.objects[ref.String()] = map[types.NamespacedName]struct{}{}
	}

	f.objects[ref.String()][obj] = struct{}{}
}

func (f *FakeTracker) TrackKind(kind schema.GroupKind, obj types.NamespacedName) {
	if f.kinds == nil {
		f.kinds = make(map[string]map[types.NamespacedName]struct{})
	}

	_, ok := f.kinds[kind.String()]
	if !ok {
		f.kinds[kind.String()] = map[types.NamespacedName]struct{}{}
	}

	f.kinds[kind.String()][obj] = struct{}{}
}

func (*FakeTracker) OnChanged(obj interface{}) {
	panic("I should not be called in tests")
}

func (f *FakeTracker) IsTracking(ref reconciler.Key, obj types.NamespacedName) bool {
	trackingObs, ok := f.objects[ref.String()]
	if !ok {
		return false
	}
	_, ok = trackingObs[obj]

	return ok
}

func (f *FakeTracker) IsTrackingKind(kind schema.GroupKind, obj types.NamespacedName) bool {
	trackingObs, ok := f.kinds[kind.String()]
	if !ok {
		return false
	}
	_, ok = trackingObs[obj]

	return ok
}

func (f FakeTracker) String() string {
	return fmt.Sprintf("%#v", f)
}
