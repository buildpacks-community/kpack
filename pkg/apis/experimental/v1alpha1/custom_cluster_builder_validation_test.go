package v1alpha1

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestCustomBuilderValidation(t *testing.T) {
	spec.Run(t, "Custom Builder Validation", testCustomBuilderValidation)
}

func testCustomBuilderValidation(t *testing.T, when spec.G, it spec.S) {
	customClusterBuilder := &CustomClusterBuilder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-builder-name",
			Namespace: "custom-builder-namespace",
		},
		Spec: CustomClusterBuilderSpec{
			CustomBuilderSpec: CustomBuilderSpec{
				Tag:          "some-registry.io/custom-builder",
				Stack:        "some-stack-ref",
				ClusterStore: "some-registry.io/store",
				Order:        nil, // No order validation
			},
			ServiceAccountRef: corev1.ObjectReference{
				Name:      "some-sa-name",
				Namespace: "some-sa-namespace",
			},
		},
	}

	when("Validate", func() {
		it("returns nil on no validation error", func() {
			assert.Nil(t, customClusterBuilder.Validate(context.TODO()))
		})

		assertValidationError := func(ccb *CustomClusterBuilder, expectedError *apis.FieldError) {
			t.Helper()
			err := ccb.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing service account name", func() {
			customClusterBuilder.Spec.ServiceAccountRef.Name = ""
			assertValidationError(customClusterBuilder, apis.ErrMissingField("name").ViaField("spec", "serviceAccountRef"))
		})

		it("missing service account namespace", func() {
			customClusterBuilder.Spec.ServiceAccountRef.Namespace = ""
			assertValidationError(customClusterBuilder, apis.ErrMissingField("namespace").ViaField("spec", "serviceAccountRef"))
		})
	})
}
