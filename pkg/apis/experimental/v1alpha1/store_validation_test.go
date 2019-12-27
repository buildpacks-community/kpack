package v1alpha1

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestStoreValidation(t *testing.T) {
	spec.Run(t, "Store Validation", testStoreValidation)
}

func testStoreValidation(t *testing.T, when spec.G, it spec.S) {
	store := &Store{
		ObjectMeta: metav1.ObjectMeta{
			Name: "store-name",
		},
		Spec: StoreSpec{
			Sources: []StoreImage{
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
			assert.Nil(t, store.Validate(context.TODO()))
		})

		assertValidationError := func(store *Store, expectedError *apis.FieldError) {
			t.Helper()
			err := store.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing field sources", func() {
			store.Spec.Sources = nil
			assertValidationError(store, apis.ErrMissingField("sources").ViaField("spec"))
		})

		it("sources should contain a fully qualified digest", func() {
			store.Spec.Sources = append(store.Spec.Sources, StoreImage{Image: "some-registry.io/store-image-4"})
			assertValidationError(store, apis.ErrInvalidArrayValue(store.Spec.Sources[3], "sources", 3).ViaField("spec"))
		})
	})
}
