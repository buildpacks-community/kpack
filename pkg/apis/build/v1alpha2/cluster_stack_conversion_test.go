package v1alpha2

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

func TestClusterStackConversion(t *testing.T) {
	spec.Run(t, "testClusterStackConversion", testClusterStackConversion)
}

func testClusterStackConversion(t *testing.T, when spec.G, it spec.S) {
	when("converting to and from v1alpha1", func() {
		v1alpha2ClusterStack := &ClusterStack{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-clusterstack",
			},
			Spec: ClusterStackSpec{
				BuildImage: ClusterStackSpecImage{
					Image: "some-build-image",
				},
				RunImage: ClusterStackSpecImage{
					Image: "some-run-image",
				},
				ServiceAccountRef: &corev1.ObjectReference{
					Kind:      "service-account",
					Namespace: "some-namespace",
					Name:      "some-service-account",
				},
			},
			Status: ClusterStackStatus{
				ResolvedClusterStack: ResolvedClusterStack{
					Id: "some-id",
					BuildImage: ClusterStackStatusImage{
						LatestImage: "some-latest-build-image",
						Image:       "some-build-image",
					},
					RunImage: ClusterStackStatusImage{
						LatestImage: "some-latest-run-image",
						Image:       "some-run-image",
					},
					Mixins:  []string{"cowsay", "ionCannon2.4", "forkbomb"},
					UserID:  0,
					GroupID: 0,
				},
			},
		}

		v1alpha1ClusterStack := &v1alpha1.ClusterStack{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-clusterstack",
			},
			Spec: v1alpha1.ClusterStackSpec{
				BuildImage: v1alpha1.ClusterStackSpecImage{
					Image: "some-build-image",
				},
				RunImage: v1alpha1.ClusterStackSpecImage{
					Image: "some-run-image",
				},
			},
			Status: v1alpha1.ClusterStackStatus{
				ResolvedClusterStack: v1alpha1.ResolvedClusterStack{
					Id: "some-id",
					BuildImage: v1alpha1.ClusterStackStatusImage{
						LatestImage: "some-latest-build-image",
						Image:       "some-build-image",
					},
					RunImage: v1alpha1.ClusterStackStatusImage{
						LatestImage: "some-latest-run-image",
						Image:       "some-run-image",
					},
					Mixins:  []string{"cowsay", "ionCannon2.4", "forkbomb"},
					UserID:  0,
					GroupID: 0,
				},
			},
		}

		it("successfully converts between api versions", func() {

			testV1alpha1ClusterStack := &v1alpha1.ClusterStack{}
			err := v1alpha2ClusterStack.ConvertTo(context.TODO(), testV1alpha1ClusterStack)
			require.NoError(t, err)
			require.Equal(t, v1alpha1ClusterStack, testV1alpha1ClusterStack)

			testV1alpha2ClusterStack := &ClusterStack{}
			err = testV1alpha2ClusterStack.ConvertFrom(context.TODO(), v1alpha1ClusterStack)
			v1alpha2ClusterStack.Spec.ServiceAccountRef = nil
			require.NoError(t, err)
			require.Equal(t, testV1alpha2ClusterStack, v1alpha2ClusterStack)
		})
	})
}
