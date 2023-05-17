package tracker_test

import (
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/pivotal/kpack/pkg/tracker"
)

func TestTracker(t *testing.T) {
	spec.Run(t, "Test Tracker", testTracker)
}

func testTracker(t *testing.T, when spec.G, it spec.S) {
	when("#Track", func() {
		var (
			reconcilerName = types.NamespacedName{
				Namespace: "some-other-namespace",
				Name:      "call-me-when-builder-changes",
			}
		)
		when("tracking a namespace scoped object", func() {
			var (
				wasCalledWith types.NamespacedName

				builder = &buildapi.Builder{
					ObjectMeta: v1.ObjectMeta{
						Name:      "some-name",
						Namespace: "some-namespace",
					},
				}

				otherBuilder = &buildapi.Builder{
					ObjectMeta: v1.ObjectMeta{
						Name:      "some-other-name",
						Namespace: "some-other-namespace",
					},
				}
			)
			it("calls the callback when OnChanged is called", func() {
				track := tracker.New(func(key types.NamespacedName) {
					wasCalledWith = key
				}, 5*time.Minute)

				track.Track(reconciler.KeyForObject(builder), reconcilerName)

				track.OnChanged(builder)

				require.Equal(t, wasCalledWith, reconcilerName)
			})

			it("doesn't do anything if OnChanged is called for a different object", func() {
				var wasCalledWith types.NamespacedName
				track := tracker.New(func(key types.NamespacedName) {
					wasCalledWith = key
				}, 5*time.Minute)

				track.Track(reconciler.KeyForObject(builder), reconcilerName)

				track.OnChanged(otherBuilder)

				require.Equal(t, wasCalledWith, types.NamespacedName{})
			})
		})

		when("tracking a cluster scoped object", func() {
			var (
				wasCalledWith  types.NamespacedName
				clusterBuilder = &buildapi.ClusterBuilder{
					ObjectMeta: v1.ObjectMeta{
						Name: "some-name",
					},
					TypeMeta: v1.TypeMeta{
						Kind:       "ClusterBuilder",
						APIVersion: "kpack.io/v1alpha2",
					},
				}

				otherClusterBuilder = &buildapi.ClusterBuilder{
					ObjectMeta: v1.ObjectMeta{
						Name: "some-other-name",
					},
					TypeMeta: v1.TypeMeta{
						Kind:       "ClusterBuilder",
						APIVersion: "kpack.io/v1alpha2",
					},
				}
			)
			it("calls the callback when OnChanged is called", func() {
				track := tracker.New(func(key types.NamespacedName) {
					wasCalledWith = key
				}, 5*time.Minute)

				track.Track(reconciler.KeyForObject(clusterBuilder), reconcilerName)

				track.OnChanged(clusterBuilder)

				require.Equal(t, wasCalledWith, reconcilerName)
			})

			it("doesn't do anything if OnChanged is called for a different object", func() {
				track := tracker.New(func(key types.NamespacedName) {
					wasCalledWith = key
				}, 5*time.Minute)

				track.Track(reconciler.KeyForObject(clusterBuilder), reconcilerName)

				track.OnChanged(otherClusterBuilder)

				require.Equal(t, wasCalledWith, types.NamespacedName{})
			})
		})

		when("tracking a group kind", func() {
			var (
				wasCalledWith types.NamespacedName
				groupKind     = schema.GroupKind{
					Group: "kpack.io",
					Kind:  "Builder",
				}
			)
			it("calls the callback when OnChanged is called", func() {
				track := tracker.New(func(key types.NamespacedName) {
					wasCalledWith = key
				}, 5*time.Minute)

				builder := &buildapi.Builder{
					ObjectMeta: v1.ObjectMeta{
						Name: "some-name",
					},
					TypeMeta: v1.TypeMeta{
						Kind:       "Builder",
						APIVersion: "kpack.io/v1alpha2",
					},
				}

				track.TrackKind(groupKind, reconcilerName)

				track.OnChanged(builder)

				require.Equal(t, wasCalledWith, reconcilerName)
			})

			it("doesn't do anything if OnChanged is called for a different kind", func() {
				track := tracker.New(func(key types.NamespacedName) {
					wasCalledWith = key
				}, 5*time.Minute)

				clusterBuilder := &buildapi.ClusterBuilder{
					ObjectMeta: v1.ObjectMeta{
						Name: "some-name",
					},
					TypeMeta: v1.TypeMeta{
						Kind:       "ClusterBuilder",
						APIVersion: "kpack.io/v1alpha2",
					},
				}

				track.TrackKind(groupKind, reconcilerName)

				track.OnChanged(clusterBuilder)

				require.Equal(t, wasCalledWith, types.NamespacedName{})
			})

			it("supports multiple reconcilers tracking the same kind", func() {
				calledWith := []types.NamespacedName{}
				track := tracker.New(func(key types.NamespacedName) {
					calledWith = append(calledWith, key)
				}, 5*time.Minute)

				builder := &buildapi.Builder{
					ObjectMeta: v1.ObjectMeta{
						Name: "some-name",
					},
					TypeMeta: v1.TypeMeta{
						Kind:       "Builder",
						APIVersion: "kpack.io/v1alpha2",
					},
				}

				secondReconciler := types.NamespacedName{Name: "second reconciler", Namespace: "some namespace"}

				track.TrackKind(groupKind, reconcilerName)
				track.TrackKind(groupKind, secondReconciler)

				track.OnChanged(builder)

				require.Len(t, calledWith, 2)
				require.Contains(t, calledWith, reconcilerName)
				require.Contains(t, calledWith, secondReconciler)
			})
		})

		when("tracking expires", func() {
			var (
				wasCalledWith types.NamespacedName
				builder       = &buildapi.Builder{
					ObjectMeta: v1.ObjectMeta{
						Name:      "some-name",
						Namespace: "some-namespace",
					},
					TypeMeta: v1.TypeMeta{
						Kind:       "Builder",
						APIVersion: "kpack.io/v1alpha2",
					},
				}
				groupKind = schema.GroupKind{
					Group: "kpack.io",
					Kind:  "Builder",
				}
			)
			it("doesn't call the OnChanged for object", func() {
				track := tracker.New(func(key types.NamespacedName) {
					wasCalledWith = key
				}, 0)

				track.TrackKind(groupKind, reconcilerName)

				track.OnChanged(builder)

				require.Equal(t, wasCalledWith, types.NamespacedName{})
			})

			it("doesn't call the OnChanged for kind ", func() {
				track := tracker.New(func(key types.NamespacedName) {
					wasCalledWith = key
				}, 0)

				track.TrackKind(groupKind, reconcilerName)

				track.OnChanged(builder)

				require.Equal(t, wasCalledWith, types.NamespacedName{})
			})
		})
	})
}
