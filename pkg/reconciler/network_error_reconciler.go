package reconciler

import (
	"context"

	"github.com/pkg/errors"
	"knative.dev/pkg/controller"
)

type NetworkErrorReconciler struct {
	Reconciler controller.Reconciler
}

func (r *NetworkErrorReconciler) Reconcile(ctx context.Context, key string) error {
	if err := r.Reconciler.Reconcile(ctx, key); err != nil {
		var networkError *NetworkError
		if errors.As(err, &networkError) {
			// Re-queue the key if it's a network error.
			return err
		}
		return controller.NewPermanentError(err)
	}
	return nil
}

type NetworkError struct {
	Err error
}

func (e *NetworkError) Error() string {
	return e.Err.Error()
}
