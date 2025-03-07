package reconciler_test

import (
	"testing"
	"time"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/reconciler"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFilterDeletionTimestamp(t *testing.T) {
	spec.Run(t, "FilterDeletionTimestamp", testFilterDeletionTimestamp)
}

func testFilterDeletionTimestamp(t *testing.T, when spec.G, it spec.S) {
	when("#FilterDeletionTimestamp", func() {
		it("returns true", func() {
			require.True(t, reconciler.FilterDeletionTimestamp(&buildapi.Build{}))
		})

		when("object is deleted", func() {
			it("returns false", func() {
				require.False(t, reconciler.FilterDeletionTimestamp(buildapi.Build{
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &metav1.Time{Time: time.Now()},
					},
				}))
			})
		})

		when("not an object", func() {
			it("returns false", func() {
				require.False(t, reconciler.FilterDeletionTimestamp("not an object"))
			})
		})
	})
}
