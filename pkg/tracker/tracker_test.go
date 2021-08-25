package tracker_test

import (
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/tracker"
)

func TestTracker(t *testing.T) {
	spec.Run(t, "Test Tracker", testTracker)
}

func testTracker(t *testing.T, when spec.G, it spec.S) {
	when("#Track", func() {
		when("tracking a namespace scoped object", func() {
			it("calls the callback when OnChanged is called", func() {
				var wasCalledWith types.NamespacedName
				track := tracker.New(func(key types.NamespacedName) {
					wasCalledWith = key
				}, 5*time.Minute)

				builder := &buildapi.Builder{
					ObjectMeta: v1.ObjectMeta{
						Name:      "some-name",
						Namespace: "some-namespace",
					},
				}
				err := track.Track(&builder.ObjectMeta, types.NamespacedName{
					Namespace: "some-other-namespace",
					Name:      "call-me-when-builder-changes",
				})
				require.NoError(t, err)

				track.OnChanged(builder)

				require.Equal(t, wasCalledWith, types.NamespacedName{
					Namespace: "some-other-namespace",
					Name:      "call-me-when-builder-changes",
				})
			})
		})

		when("tracking a cluster scoped object", func() {
			it("calls the callback when OnChanged is called", func() {
				var wasCalledWith types.NamespacedName
				track := tracker.New(func(key types.NamespacedName) {
					wasCalledWith = key
				}, 5*time.Minute)

				clusterBuilder := &buildapi.ClusterBuilder{
					ObjectMeta: v1.ObjectMeta{
						Name: "some-name",
					},
				}
				err := track.Track(&clusterBuilder.ObjectMeta, types.NamespacedName{
					Namespace: "some-other-namespace",
					Name:      "call-me-when-builder-changes",
				})
				require.NoError(t, err)

				track.OnChanged(clusterBuilder)

				require.Equal(t, wasCalledWith, types.NamespacedName{
					Namespace: "some-other-namespace",
					Name:      "call-me-when-builder-changes",
				})
			})
		})
	})
}
