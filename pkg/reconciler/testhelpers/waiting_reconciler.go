package testhelpers

import (
	"context"
	"errors"
	knCtrl "github.com/knative/pkg/controller"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"k8s.io/client-go/tools/cache"
)

func SyncWaitingReconciler(informerFactory externalversions.SharedInformerFactory, reconciler knCtrl.Reconciler, hasSynced ...func() bool) knCtrl.Reconciler {
	return &waitingInformerDecorator{informerFactory, reconciler, hasSynced}
}

type waitingInformerDecorator struct {
	informerFactory externalversions.SharedInformerFactory
	reconciler      knCtrl.Reconciler
	hasSynced       []func() bool
}

func (c *waitingInformerDecorator) Reconcile(ctx context.Context, key string) error {
	stopChan := make(chan struct{})
	defer close(stopChan)
	c.informerFactory.Start(stopChan)

	for _, synced := range c.hasSynced {
		if ok := cache.WaitForCacheSync(make(<-chan struct{}), synced); !ok {
			return errors.New("couldn't sync")
		}
	}
	return c.reconciler.Reconcile(ctx, key)
}

type RebuildingReconciler func() knCtrl.Reconciler

func (r RebuildingReconciler) Reconcile(ctx context.Context, key string) error {
	return r().Reconcile(ctx, key)
}
