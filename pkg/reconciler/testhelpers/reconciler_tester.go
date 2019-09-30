package testhelpers

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func ReconcilerTester(t *testing.T, factory rtesting.Factory) SpecReconcilerTester {
	return SpecReconcilerTester{
		t:       t,
		factory: factory,
	}
}

type SpecReconcilerTester struct {
	t       *testing.T
	factory rtesting.Factory
}

func (rt SpecReconcilerTester) Test(test rtesting.TableRow) {
	rt.t.Helper()
	originObjects := []runtime.Object{}
	for _, obj := range test.Objects {
		originObjects = append(originObjects, obj.DeepCopyObject())
	}

	test.Test(rt.t, rt.factory)

	// Validate cached objects do not get soiled after controller loops
	if diff := cmp.Diff(originObjects, test.Objects, safeDeployDiff, cmpopts.EquateEmpty()); diff != "" {
		rt.t.Errorf("Unexpected objects in test %s (-want, +got): %v", test.Name, diff)
	}
}

var (
	safeDeployDiff = cmpopts.IgnoreUnexported(resource.Quantity{})
)
