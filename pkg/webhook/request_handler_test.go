package webhook_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattbaird/jsonpatch"
	"github.com/pkg/errors"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/admission/v1beta1"

	"github.com/pivotal/kpack/pkg/webhook"
)

func TestRequestHandler(t *testing.T) {
	spec.Run(t, "Test Request Handler", testRequestHandler)
}

const mutatePodRequestJson = `{
  "apiVersion": "admission.k8s.io/v1beta1",
  "kind": "AdmissionReview",
  "request": {
    "uid": "705ab4f5-6393-11e8-b7cc-42010a800002",
    "kind": {"group":"autoscaling","version":"v1","kind":"Scale"},
    "requestKind": {"group":"autoscaling","version":"v1","kind":"Scale"},
    "name": "my-deployment",
    "namespace": "my-namespace",
    "operation": "CREATE",
    "object": {"apiVersion":"autoscaling/v1","kind":"Scale"}
  }
}`

func testRequestHandler(t *testing.T, when spec.G, it spec.S) {
	when("/mutate", func() {
		it("invokes the apply function", func() {
			var request *http.Request
			var response *httptest.ResponseRecorder
			var err error
			operation := jsonpatch.JsonPatchOperation{
				Operation: "add",
				Path:      "/containers/1/env",
				Value:     nil,
			}

			response = httptest.NewRecorder()
			request, err = http.NewRequest("POST", "/mutate", bytes.NewBuffer([]byte(mutatePodRequestJson)))
			require.NoError(t, err)

			fakeApplyFunc := func(*v1beta1.AdmissionRequest) ([]jsonpatch.JsonPatchOperation, error) {
				return []jsonpatch.JsonPatchOperation{operation}, nil
			}

			handler := webhook.RequestHandler{ApplyFunc: fakeApplyFunc}

			handler.ServeHTTP(response, request)

			require.Equal(t, 200, response.Code)
			assert.JSONEq(t, `{
  "response": {
    "uid": "705ab4f5-6393-11e8-b7cc-42010a800002",
    "allowed": true,
    "patch": "W3sib3AiOiJhZGQiLCJwYXRoIjoiL2NvbnRhaW5lcnMvMS9lbnYiLCJ2YWx1ZSI6bnVsbH1d"
  }
}`, response.Body.String())
		})

		it("it returns allow false with error message if admit returns an error", func() {
			var request *http.Request
			var response *httptest.ResponseRecorder
			var err error

			response = httptest.NewRecorder()
			request, err = http.NewRequest("POST", "/mutate", bytes.NewBuffer([]byte(mutatePodRequestJson)))
			require.NoError(t, err)

			fakeApplyFunc := func(*v1beta1.AdmissionRequest) ([]jsonpatch.JsonPatchOperation, error) {
				return nil, errors.New("failed to apply")
			}

			handler := webhook.RequestHandler{ApplyFunc: fakeApplyFunc}

			handler.ServeHTTP(response, request)

			require.Equal(t, 200, response.Code)
			assert.JSONEq(t, `{
  "response": {
    "uid": "705ab4f5-6393-11e8-b7cc-42010a800002",
    "allowed": false,
    "status": {
      "metadata": {},
      "message": "failed to apply"
    }
  }
}
`, response.Body.String())
		})

		it("it returns methodNotAllowed if request type is not a POST", func() {
			var request *http.Request
			var response *httptest.ResponseRecorder
			var err error

			response = httptest.NewRecorder()
			request, err = http.NewRequest("PATCH", "/mutate", bytes.NewBuffer([]byte(mutatePodRequestJson)))
			require.NoError(t, err)

			fakeApplyFunc := func(*v1beta1.AdmissionRequest) ([]jsonpatch.JsonPatchOperation, error) {
				return nil, nil
			}

			handler := webhook.RequestHandler{ApplyFunc: fakeApplyFunc}

			handler.ServeHTTP(response, request)

			require.Equal(t, 405, response.Code)
		})
	})
}
