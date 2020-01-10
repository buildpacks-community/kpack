package v1alpha1

import (
	"context"
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestStackValidation(t *testing.T) {
	spec.Run(t, "Stack Validation", testStackValidation)
}

func testStackValidation(t *testing.T, when spec.G, it spec.S) {
	stack := &Stack{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stack-name",
			Namespace: "stack-namespace",
		},
		Spec: StackSpec{
			Id: "io.my.stack",
			BuildImage: StackSpecImage{
				Image: "gcr.io/my/buildimage",
			},
			RunImage: StackSpecImage{
				Image: "gcr.io/my/runimage",
			},
		},
	}

	when("Validate", func() {
		assertValidationError := func(stack *Stack, expectedError *apis.FieldError) {
			t.Helper()
			err := stack.Validate(context.TODO())
			assert.EqualError(t, err, expectedError.Error())
		}

		it("returns nil on no validation error", func() {
			assert.Nil(t, stack.Validate(context.TODO()))
		})

		it("missing id", func() {
			stack.Spec.Id = ""

			assertValidationError(stack, apis.ErrMissingField("id").ViaField("spec"))
		})

		it("invalid build image", func() {
			stack.Spec.BuildImage.Image = "@INAVALID!"

			assertValidationError(stack, apis.ErrInvalidValue("@INAVALID!", "image").ViaField("buildImage").ViaField("spec"))
		})

		it("missing build image", func() {
			stack.Spec.BuildImage.Image = ""

			assertValidationError(stack, apis.ErrMissingField("image").ViaField("buildImage").ViaField("spec"))
		})

		it("invalid run image", func() {
			stack.Spec.RunImage.Image = "@INAVALID!"

			assertValidationError(stack, apis.ErrInvalidValue("@INAVALID!", "image").ViaField("runImage").ViaField("spec"))
		})

		it("missing run image", func() {
			stack.Spec.RunImage.Image = ""

			assertValidationError(stack, apis.ErrMissingField("image").ViaField("runImage").ViaField("spec"))
		})
	})
}
