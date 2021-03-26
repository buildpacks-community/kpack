package logs

import (
	"context"
	"fmt"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
)

type watchBuild struct {
	buildName   string
	kpackClient versioned.Interface
	namespace   string
	context     context.Context
}

func (l *watchBuild) Watch(options v1.ListOptions) (watch.Interface, error) {
	options.FieldSelector = fmt.Sprintf("metadata.name=%s", l.buildName)

	return l.kpackClient.KpackV1alpha1().Builds(l.namespace).Watch(l.context, options)
}
