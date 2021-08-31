package logs

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
)

type watchOneImage struct {
	kpackClient versioned.Interface
	image       *buildapi.Image
	ctx         context.Context
}

func (w watchOneImage) Watch(options v1.ListOptions) (watch.Interface, error) {
	options.FieldSelector = fmt.Sprintf("metadata.name=%s", w.image.Name)
	return w.kpackClient.KpackV1alpha2().Images(w.image.Namespace).Watch(w.ctx, options)
}
