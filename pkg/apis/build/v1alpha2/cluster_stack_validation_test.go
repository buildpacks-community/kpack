package v1alpha2

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestClusterStackValidation(t *testing.T) {
	spec.Run(t, "Stack Validation", testClusterStackValidation)
}

func testClusterStackValidation(t *testing.T, when spec.G, it spec.S) {
	clusterStack := &ClusterStack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack-name",
			Namespace: "stack-namespace",
		},
		Spec: ClusterStackSpec{
			Id: "io.my.stack",
			BuildImage: ClusterStackSpecImage{
				Image: "gcr.io/my/buildimage",
			},
			RunImage: ClusterStackSpecImage{
				Image: "gcr.io/my/runimage",
			},
		},
	}

	when("Validate", func() {
		assertValidationError := func(clusterStack *ClusterStack, expectedError *apis.FieldError) {
			t.Helper()
			err := clusterStack.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("returns nil on no validation error", func() {
			assert.Nil(t, clusterStack.Validate(context.TODO()))
		})

		it("missing id", func() {
			clusterStack.Spec.Id = ""

			assertValidationError(clusterStack, apis.ErrMissingField("id").ViaField("spec"))
		})

		it("invalid build image", func() {
			clusterStack.Spec.BuildImage.Image = "@INAVALID!"

			assertValidationError(clusterStack, apis.ErrInvalidValue("@INAVALID!", "image").ViaField("buildImage").ViaField("spec"))
		})

		it("missing build image", func() {
			clusterStack.Spec.BuildImage.Image = ""

			assertValidationError(clusterStack, apis.ErrMissingField("image").ViaField("buildImage").ViaField("spec"))
		})

		it("invalid run image", func() {
			clusterStack.Spec.RunImage.Image = "@INAVALID!"

			assertValidationError(clusterStack, apis.ErrInvalidValue("@INAVALID!", "image").ViaField("runImage").ViaField("spec"))
		})

		it("missing run image", func() {
			clusterStack.Spec.RunImage.Image = ""

			assertValidationError(clusterStack, apis.ErrMissingField("image").ViaField("runImage").ViaField("spec"))
		})

		it("missing namespace in serviceAccountRef", func() {
			clusterStack.Spec.ServiceAccountRef = &corev1.ObjectReference{Name: "test"}

			assertValidationError(clusterStack, apis.ErrMissingField("namespace").ViaField("serviceAccountRef").ViaField("spec"))
		})

		it("missing name in serviceAccountRef", func() {
			clusterStack.Spec.ServiceAccountRef = &corev1.ObjectReference{Namespace: "test"}

			assertValidationError(clusterStack, apis.ErrMissingField("name").ViaField("serviceAccountRef").ViaField("spec"))
		})
	})
}
