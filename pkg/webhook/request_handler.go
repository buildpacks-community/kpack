package webhook

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/mattbaird/jsonpatch"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

type RequestHandler struct {
	ApplyFunc func(*v1beta1.AdmissionRequest) ([]jsonpatch.JsonPatchOperation, error)
}

func (rh *RequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	admissionReview := &v1beta1.AdmissionReview{}
	_, _, err = universalDeserializer.Decode(body, nil, admissionReview)
	if err != nil {
		log.Printf(err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	review, err := rh.apply(admissionReview)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}

	bytes, err := json.Marshal(review)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(err.Error()))
		return
	}
	_, _ = w.Write(bytes)
}

func (rh *RequestHandler) apply(admissionReview *v1beta1.AdmissionReview) (*v1beta1.AdmissionReview, error) {
	operations, err := rh.ApplyFunc(admissionReview.Request)
	if err != nil {
		return &v1beta1.AdmissionReview{
			Response: &v1beta1.AdmissionResponse{
				UID:     admissionReview.Request.UID,
				Allowed: false,
				Result: &metav1.Status{
					Message: err.Error(),
				},
			},
		}, nil
	}

	patchBytes, err := json.Marshal(operations)
	if err != nil {
		return nil, err
	}

	return &v1beta1.AdmissionReview{
		Response: &v1beta1.AdmissionResponse{
			UID:     admissionReview.Request.UID,
			Allowed: true,
			Patch:   patchBytes,
		},
	}, nil
}

var universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
