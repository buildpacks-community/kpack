package lifecycle

import (
	"context"
	"fmt"
	"github.com/pivotal/kpack/pkg/tracker"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/types"
	coreinformers "k8s.io/client-go/informers/core/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"

	"github.com/pivotal/kpack/pkg/reconciler"
)

type LifecycleProvider interface {
	UpdateImage(cm *corev1.ConfigMap) error
}

func NewController(
	ctx context.Context,
	opt reconciler.Options,
	k8sClient k8sclient.Interface,
	configmapName string,
	configMapInformer coreinformers.ConfigMapInformer,
	lifecycleProvider LifecycleProvider,
) *controller.Impl {
	key := types.NamespacedName{
		Namespace: system.Namespace(),
		Name:      configmapName,
	}

	c := &Reconciler{
		Key:               key,
		ConfigMapLister:   configMapInformer.Lister(),
		K8sClient:         k8sClient,
		LifecycleProvider: lifecycleProvider,
	}

	const queueName = "lifecycle"
	impl := controller.NewContext(ctx, c, controller.ControllerOptions{WorkQueueName: queueName, Logger: logging.FromContext(ctx).Named(queueName)})

	// Reconcile when the lifecycle configmap changes.
	configMapInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(key.Namespace, key.Name),
		Handler:    controller.HandleAll(impl.Enqueue),
	})

	c.Tracker = tracker.New(impl.EnqueueKey, opt.TrackerResyncPeriod())

	return impl
}

type Reconciler struct {
	Key               types.NamespacedName
	ConfigMapLister   corelisters.ConfigMapLister
	Tracker           reconciler.Tracker
	K8sClient         k8sclient.Interface
	LifecycleProvider LifecycleProvider
}

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	namespace, configMapName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("failed splitting meta namespace key: %s", err)
	}

	lifecycleConfigMap, err := c.ConfigMapLister.ConfigMaps(namespace).Get(configMapName)
	if err != nil {
		return err
	}

	return c.LifecycleProvider.UpdateImage(lifecycleConfigMap)
}
