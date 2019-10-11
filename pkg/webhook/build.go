package webhook

import (
	"encoding/json"
	"log"

	"github.com/mattbaird/jsonpatch"
	"github.com/pkg/errors"
	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

var (
	buildResource = metav1.GroupVersionResource{Version: "v1alpha1", Resource: "builds", Group: "build.pivotal.io"}
)


type BuildDefaults struct {
	ServiceAccount string
}

func (b BuildDefaults) Apply(request *v1beta1.AdmissionRequest) ([]jsonpatch.JsonPatchOperation, error) {
	if request.Resource != buildResource {
		log.Printf("expect resource to be %s", buildResource)
		return nil, nil
	}

	raw := request.Object.Raw
	build := v1alpha1.Build{}
	if _, _, err := universalDeserializer.Decode(raw, nil, &build); err != nil {
		return nil, errors.Errorf("could not deserialize build object: %v", err)
	}

	if build.Spec.ServiceAccount != "" {
		return nil, nil
	}

	build.Spec.ServiceAccount = b.ServiceAccount

	updatedbuildRaw, err := json.Marshal(build)
	if err != nil {
		return nil, err
	}
	return jsonpatch.CreatePatch(raw, updatedbuildRaw)
}