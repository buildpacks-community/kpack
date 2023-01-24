package v1alpha2

import (
	"context"
	"testing"

	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestClusterBuildpackValidation(t *testing.T) {
	spec.Run(t, "ClusterBuildpack Validation", testClusterBuildpackValidation)
}

func testClusterBuildpackValidation(t *testing.T, when spec.G, it spec.S) {
	clusterBuildpack := &ClusterBuildpack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-builder-name",
			Namespace: "custom-builder-namespace",
		},
		Spec: ClusterBuildpackSpec{
			Source: corev1alpha1.ImageSource{
				Image: "some-registry.io/store-image-1@sha256:78c1b9419976227e05be9d243b7fa583bea44a5258e52018b2af4cdfe23d148d",
			},
			ServiceAccountRef: &corev1.ObjectReference{
				Name:      "some-sa-name",
				Namespace: "some-sa-namespace",
			},
		},
	}

	when("Validate", func() {
		it("returns nil on no validation error", func() {
			assert.Nil(t, clusterBuildpack.Validate(context.TODO()))
		})

		assertValidationError := func(clusterBuildpack *ClusterBuildpack, expectedError *apis.FieldError) {
			t.Helper()
			err := clusterBuildpack.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("missing source image", func() {
			clusterBuildpack.Spec.Source.Image = ""
			assertValidationError(clusterBuildpack, apis.ErrMissingField("image").ViaField("spec", "source"))
		})

		it("invalid source image", func() {
			clusterBuildpack.Spec.Source.Image = "ftp//invalid/tag@@"

			assertValidationError(clusterBuildpack,
				apis.ErrInvalidValue(clusterBuildpack.Spec.Source.Image, "image").ViaField("spec", "source"),
			)
		})

		it("missing namespace in serviceAccountRef", func() {
			clusterBuildpack.Spec.ServiceAccountRef = &corev1.ObjectReference{Name: "test"}

			assertValidationError(clusterBuildpack, apis.ErrMissingField("namespace").ViaField("serviceAccountRef").ViaField("spec"))
		})

		it("missing name in serviceAccountRef", func() {
			clusterBuildpack.Spec.ServiceAccountRef = &corev1.ObjectReference{Namespace: "test"}

			assertValidationError(clusterBuildpack, apis.ErrMissingField("name").ViaField("serviceAccountRef").ViaField("spec"))
		})

	})
}
