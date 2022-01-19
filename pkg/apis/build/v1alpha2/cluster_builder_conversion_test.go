package v1alpha2

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestClusterBuilderConversion(t *testing.T) {
	spec.Run(t, "testClusterBuilderConversion", testClusterBuilderConversion)
}

func testClusterBuilderConversion(t *testing.T, when spec.G, it spec.S) {
	when("converting to and from v1alpha1", func() {
		v1alpha2ClusterBuilder := &ClusterBuilder{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-builder",
				Namespace:   "some-namespace",
				Annotations: map[string]string{"some-key": "some-value"},
			},
			Spec: ClusterBuilderSpec{
				BuilderSpec: BuilderSpec{
					Tag: "some-tag",
					Stack: corev1.ObjectReference{
						Kind: "ClusterStack",
						Name: "some-stack",
					},
					Store: corev1.ObjectReference{
						Kind: "ClusterStore",
						Name: "some-store",
					},
					Order: []corev1alpha1.OrderEntry{corev1alpha1.OrderEntry{
						Group: []corev1alpha1.BuildpackRef{{
							BuildpackInfo: corev1alpha1.BuildpackInfo{
								Id: "org.some-buildpack",
							},
						}},
					}},
				},
				ServiceAccountRef: corev1.ObjectReference{
					Namespace: "some-namespace",
					Name:      "some-service-account",
				},
			},
			Status: BuilderStatus{
				Stack: corev1alpha1.BuildStack{},
			},
		}

		v1alpha1ClusterBuilder := &v1alpha1.ClusterBuilder{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-builder",
				Namespace:   "some-namespace",
				Annotations: map[string]string{"some-key": "some-value"},
			},
			Spec: v1alpha1.ClusterBuilderSpec{
				BuilderSpec: v1alpha1.BuilderSpec{
					Tag: "some-tag",
					Stack: corev1.ObjectReference{
						Kind: "ClusterStack",
						Name: "some-stack",
					},
					Store: corev1.ObjectReference{
						Kind: "ClusterStore",
						Name: "some-store",
					},
					Order: []corev1alpha1.OrderEntry{corev1alpha1.OrderEntry{
						Group: []corev1alpha1.BuildpackRef{{
							BuildpackInfo: corev1alpha1.BuildpackInfo{
								Id: "org.some-buildpack",
							},
						}},
					}},
				},
				ServiceAccountRef: corev1.ObjectReference{
					Namespace: "some-namespace",
					Name:      "some-service-account",
				},
			},
			Status: v1alpha1.BuilderStatus{
				Stack: corev1alpha1.BuildStack{},
			},
		}

		it("successfully converts between api versions", func() {

			testV1alpha1ClusterBuilder := &v1alpha1.ClusterBuilder{}
			err := v1alpha2ClusterBuilder.ConvertTo(context.TODO(), testV1alpha1ClusterBuilder)
			require.NoError(t, err)
			require.Equal(t, v1alpha1ClusterBuilder, testV1alpha1ClusterBuilder)

			testV1alpha2ClusterBuilder := &ClusterBuilder{}
			err = testV1alpha2ClusterBuilder.ConvertFrom(context.TODO(), v1alpha1ClusterBuilder)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2ClusterBuilder, v1alpha2ClusterBuilder)
		})
	})
}
