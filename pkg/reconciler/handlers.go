package reconciler

import (
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
)

func Handler(h func(interface{})) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    h,
		UpdateFunc: controller.PassNew(h),
		DeleteFunc: h,
	}
}
