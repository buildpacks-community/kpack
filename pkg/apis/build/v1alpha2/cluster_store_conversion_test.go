package v1alpha2

import (
	"context"
	"testing"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClusterStoreConversion(t *testing.T) {
	spec.Run(t, "testClusterStackConversion", testClusterStoreConversion)
}

func testClusterStoreConversion(t *testing.T, when spec.G, it spec.S) {
	when("converting to and from v1alpha1", func() {
		v1alpha2ClusterStore := &ClusterStore{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-cluster-store",
				Annotations: map[string]string{"some-key": "some-value"},
			},
			Spec: ClusterStoreSpec{
				Sources: []corev1alpha1.StoreImage{{"some-image"}, {"another-image"}},
				ServiceAccountRef: &corev1.ObjectReference{
					Namespace: "some-namespace",
					Name:      "some-service-account",
				},
			},
			Status: ClusterStoreStatus{
				Status: corev1alpha1.Status{},
				Buildpacks: []corev1alpha1.StoreBuildpack{{
					BuildpackInfo: corev1alpha1.BuildpackInfo{
						Id:      "cool-buildpack-id",
						Version: "1.23",
					},
					Buildpackage: corev1alpha1.BuildpackageInfo{
						Id:       "cool-buildpackage-id",
						Version:  "4.56",
						Homepage: "wow-what-a-site.com",
					},
					StoreImage: corev1alpha1.StoreImage{
						Image: "some-image",
					},
					DiffId:   "12345",
					Digest:   "some-digest",
					Size:     0,
					API:      "some-api-version",
					Homepage: "neopets.com",
					Order:    nil,
					Stacks:   nil,
				}},
			},
		}

		v1alpha1ClusterStore := &v1alpha1.ClusterStore{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-cluster-store",
				Annotations: map[string]string{"some-key": "some-value"},
			},
			Spec: v1alpha1.ClusterStoreSpec{
				Sources: []corev1alpha1.StoreImage{{"some-image"}, {"another-image"}},
			},
			Status: v1alpha1.ClusterStoreStatus{
				Status: corev1alpha1.Status{},
				Buildpacks: []corev1alpha1.StoreBuildpack{{
					BuildpackInfo: corev1alpha1.BuildpackInfo{
						Id:      "cool-buildpack-id",
						Version: "1.23",
					},
					Buildpackage: corev1alpha1.BuildpackageInfo{
						Id:       "cool-buildpackage-id",
						Version:  "4.56",
						Homepage: "wow-what-a-site.com",
					},
					StoreImage: corev1alpha1.StoreImage{
						Image: "some-image",
					},
					DiffId:   "12345",
					Digest:   "some-digest",
					Size:     0,
					API:      "some-api-version",
					Homepage: "neopets.com",
					Order:    nil,
					Stacks:   nil,
				}},
			},
		}

		it("successfully converts between api versions", func() {

			testV1alpha1ClusterStore := &v1alpha1.ClusterStore{}
			err := v1alpha2ClusterStore.ConvertTo(context.TODO(), testV1alpha1ClusterStore)
			require.NoError(t, err)
			require.Equal(t, v1alpha1ClusterStore, testV1alpha1ClusterStore)

			testV1alpha2ClusterStore := &ClusterStore{}
			err = testV1alpha2ClusterStore.ConvertFrom(context.TODO(), v1alpha1ClusterStore)
			v1alpha2ClusterStore.Spec.ServiceAccountRef = nil
			require.NoError(t, err)
			require.Equal(t, testV1alpha2ClusterStore, v1alpha2ClusterStore)
		})
	})
}
