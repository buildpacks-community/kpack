package logs

import (
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
)

type listAndWatchBuild struct {
	buildName   string
	kpackClient versioned.Interface
	namespace   string
}

func (l *listAndWatchBuild) List(options v1.ListOptions) (runtime.Object, error) {
	options.FieldSelector = fmt.Sprintf("metadata.name=%s", l.buildName)

	return l.kpackClient.BuildV1alpha1().Builds(l.namespace).List(options)
}

func (l *listAndWatchBuild) Watch(options v1.ListOptions) (watch.Interface, error) {
	options.FieldSelector = fmt.Sprintf("metadata.name=%s", l.buildName)

	return l.kpackClient.BuildV1alpha1().Builds(l.namespace).Watch(options)
}
