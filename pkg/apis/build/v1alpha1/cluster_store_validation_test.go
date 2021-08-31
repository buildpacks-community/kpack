package v1alpha1

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestClusterStoreValidation(t *testing.T) {
	spec.Run(t, "ClusterStore Validation", testClusterStoreValidation)
}

func testClusterStoreValidation(t *testing.T, when spec.G, it spec.S) {
	clusterStore := &ClusterStore{
		ObjectMeta: metav1.ObjectMeta{
			Name: "store-name",
		},
		Spec: ClusterStoreSpec{
			Sources: []corev1alpha1.StoreImage{
				{
					Image: "some-registry.io/store-image-1@sha256:78c1b9419976227e05be9d243b7fa583bea44a5258e52018b2af4cdfe23d148d",
				},
				{
					Image: "some-registry.io/store-image-2@sha256:78c1b9419976227e05be9d243b7fa583bea44a5258e52018b2af4cdfe23d148d",
				},
				{
					Image: "some-registry.io/store-image-3@sha256:78c1b9419976227e05be9d243b7fa583bea44a5258e52018b2af4cdfe23d148d",
				},
			},
		},
	}

	when("Validate", func() {
		it("returns nil on no validation error", func() {
			assert.Nil(t, clusterStore.Validate(context.TODO()))
		})

		assertValidationError := func(clusterStore *ClusterStore, expectedError *apis.FieldError) {
			t.Helper()
			err := clusterStore.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing field sources", func() {
			clusterStore.Spec.Sources = nil
			assertValidationError(clusterStore, apis.ErrMissingField("sources").ViaField("spec"))
		})

		it("sources should contain a valid image", func() {
			clusterStore.Spec.Sources = append(clusterStore.Spec.Sources, corev1alpha1.StoreImage{Image: "invalid image"})
			assertValidationError(clusterStore, apis.ErrInvalidArrayValue(clusterStore.Spec.Sources[3], "sources", 3).ViaField("spec"))
		})
	})
}
