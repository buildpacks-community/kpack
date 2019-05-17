package testhelpers

import (
	"context"
	"errors"
	knCtrl "github.com/knative/pkg/controller"
	"k8s.io/client-go/tools/cache"
)

func SyncWaitingReconciler(reconciler knCtrl.Reconciler, hasSynced ...func() bool) knCtrl.Reconciler {
	return &waitingInformerDecorator{reconciler, hasSynced}
}

type waitingInformerDecorator struct {
	reconciler knCtrl.Reconciler
	hasSynced  []func() bool
}

func (c *waitingInformerDecorator) Reconcile(ctx context.Context, key string) error {
	for _, synced := range c.hasSynced {
		if ok := cache.WaitForCacheSync(make(<-chan struct{}), synced); !ok {
			return errors.New("couldn't sync")
		}
	}
	return c.reconciler.Reconcile(ctx, key)
}
