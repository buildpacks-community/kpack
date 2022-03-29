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

func TestBuilderConversion(t *testing.T) {
	spec.Run(t, "testBuilderConversion", testBuilderConversion)
}

func testBuilderConversion(t *testing.T, when spec.G, it spec.S) {
	when("converting to and from v1alpha1", func() {
		v1alpha2Builder := &Builder{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-builder",
				Namespace:   "some-namespace",
				Annotations: map[string]string{"some-key": "some-value"},
			},
			Spec: NamespacedBuilderSpec{
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
				ServiceAccountName: "some-service-account",
			},
			Status: BuilderStatus{
				Status:          corev1alpha1.Status{Conditions: corev1alpha1.Conditions{{Type: "some-type"}}},
				BuilderMetadata: nil,
				Order:           nil,
				Stack: corev1alpha1.BuildStack{
					RunImage: "",
					ID:       "",
				},
				LatestImage:             "",
				ObservedStackGeneration: 0,
				ObservedStoreGeneration: 0,
				OS:                      "",
			},
		}
		v1alpha1Builder := &v1alpha1.Builder{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-builder",
				Namespace:   "some-namespace",
				Annotations: map[string]string{"some-key": "some-value"},
			},
			Spec: v1alpha1.NamespacedBuilderSpec{
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
				ServiceAccount: "some-service-account",
			},
			Status: v1alpha1.BuilderStatus{
				Status: corev1alpha1.Status{Conditions: corev1alpha1.Conditions{{Type: "some-type"}}},
				Stack:  corev1alpha1.BuildStack{},
			},
		}

		it("successfully converts between api versions", func() {

			testV1alpha1Builder := &v1alpha1.Builder{}
			err := v1alpha2Builder.ConvertTo(context.TODO(), testV1alpha1Builder)
			require.NoError(t, err)
			require.Equal(t, v1alpha1Builder, testV1alpha1Builder)

			testV1alpha2Builder := &Builder{}
			err = testV1alpha2Builder.ConvertFrom(context.TODO(), v1alpha1Builder)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2Builder, v1alpha2Builder)
		})
	})
}
