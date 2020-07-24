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

func TestClusterBuilderValidation(t *testing.T) {
	spec.Run(t, "Cluster Builder Validation", testClusterBuilderValidation)
}

func testClusterBuilderValidation(t *testing.T, when spec.G, it spec.S) {
	clusterBuilder := &ClusterBuilder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-builder-name",
			Namespace: "custom-builder-namespace",
		},
		Spec: ClusterBuilderSpec{
			BuilderSpec: BuilderSpec{
				Tag: "some-registry.io/custom-builder",
				Stack: corev1.ObjectReference{
					Kind: "ClusterStack",
					Name: "some-stack-ref",
				},
				Store: corev1.ObjectReference{
					Kind: "ClusterStore",
					Name: "some-registry.io/store",
				},
				Order: nil, // No order validation
			},
			ServiceAccountRef: corev1.ObjectReference{
				Name:      "some-sa-name",
				Namespace: "some-sa-namespace",
			},
		},
	}

	when("Validate", func() {
		it("returns nil on no validation error", func() {
			assert.Nil(t, clusterBuilder.Validate(context.TODO()))
		})

		assertValidationError := func(ccb *ClusterBuilder, expectedError *apis.FieldError) {
			t.Helper()
			err := ccb.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing service account name", func() {
			clusterBuilder.Spec.ServiceAccountRef.Name = ""
			assertValidationError(clusterBuilder, apis.ErrMissingField("name").ViaField("spec", "serviceAccountRef"))
		})

		it("missing service account namespace", func() {
			clusterBuilder.Spec.ServiceAccountRef.Namespace = ""
			assertValidationError(clusterBuilder, apis.ErrMissingField("namespace").ViaField("spec", "serviceAccountRef"))
		})
	})
}
