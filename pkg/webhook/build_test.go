package webhook_test

import (
	"encoding/json"
	"testing"

	"github.com/mattbaird/jsonpatch"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/webhook"
)

func TestEnvVars(t *testing.T) {
	spec.Run(t, "Test Build Webhooks", testEnvVars)
}

func testEnvVars(t *testing.T, when spec.G, it spec.S) {
	testBuild := &v1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "some-build",
		},
		Spec:       v1alpha1.BuildSpec{
			ServiceAccount: "some-service-account",
		},
	}

	when("#BuildDefaults", func() {
		it("does not change build when service account if present", func() {
			bytes, err := json.Marshal(testBuild)
			require.NoError(t, err)
			admissionRequest := &v1beta1.AdmissionRequest{
				Name: "testAdmissionRequest",
				Object: runtime.RawExtension{
					Raw: bytes,
				},
				Resource: metav1.GroupVersionResource{Version: "v1alpha1", Resource: "builds", Group: "build.pivotal.io"},
			}

			subject := webhook.BuildDefaults{
				ServiceAccount: "some-other-service-account",
			}

			operations, err := subject.Apply(admissionRequest)
			require.NoError(t, err)
			assert.Nil(t, operations)
		})

		it("changes build when service account if not present", func() {
			testBuild.Spec.ServiceAccount = "some-other-service-account"
			expectedBytes, err := json.Marshal(testBuild)
			require.NoError(t, err)

			testBuild.Spec.ServiceAccount = ""
			bytes, err := json.Marshal(testBuild)
			require.NoError(t, err)
			admissionRequest := &v1beta1.AdmissionRequest{
				Name: "testAdmissionRequest",
				Object: runtime.RawExtension{
					Raw: bytes,
				},
				Resource: metav1.GroupVersionResource{Version: "v1alpha1", Resource: "builds", Group: "build.pivotal.io"},
			}

			subject := webhook.BuildDefaults{
				ServiceAccount: "some-other-service-account",
			}

			operations, err := subject.Apply(admissionRequest)
			require.NoError(t, err)

			expectedPatchOperations, err := jsonpatch.CreatePatch(bytes, expectedBytes)
			require.NoError(t, err)

			assert.ElementsMatch(t, expectedPatchOperations, operations)
		})
	})
}