package testhelpers

import (
	"fmt"

	"github.com/pivotal/kpack/pkg/reconciler"
	"k8s.io/apimachinery/pkg/types"
)

type FakeTracker map[string]map[types.NamespacedName]struct{}

func (f FakeTracker) Track(ref reconciler.Key, obj types.NamespacedName) error {
	_, ok := f[ref.String()]
	if !ok {
		f[ref.String()] = map[types.NamespacedName]struct{}{}
	}

	f[ref.String()][obj] = struct{}{}
	return nil
}

func (FakeTracker) OnChanged(obj interface{}) {
	panic("I should not be called in tests")
}

func (f FakeTracker) IsTracking(ref reconciler.Key, obj types.NamespacedName) bool {
	trackingObs, ok := f[ref.String()]
	if !ok {
		return false
	}
	_, ok = trackingObs[obj]

	return ok
}

func (f FakeTracker) String() string {
	return fmt.Sprintf("%#v", f)
}
