package reconciler

import (
	"github.com/knative/pkg/controller"
	"k8s.io/client-go/tools/cache"
)

func Handler(h func(interface{})) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    h,
		UpdateFunc: controller.PassNew(h),
		DeleteFunc: h,
	}
}
