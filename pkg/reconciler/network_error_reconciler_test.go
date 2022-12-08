package reconciler

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"knative.dev/pkg/controller"
)

const (
	networkErrorMsg = "some network problem"
	otherErrorMsg   = "some other error"
)

func TestNetworkErrorReconciler(t *testing.T) {
	spec.Run(t, "Network Error Reconciler", testNetworkErrorReconciler)
}

func testNetworkErrorReconciler(t *testing.T, when spec.G, it spec.S) {

	when("#Reconcile", func() {
		when("network error", func() {
			r := &fakeErrorReconciler{err: &NetworkError{Err: errors.New(networkErrorMsg)}}
			subject := &NetworkErrorReconciler{Reconciler: r}
			it("re-throw the error", func() {
				err := subject.Reconcile(context.Background(), "whatever")

				require.Error(t, err)

				var networkError *NetworkError
				assert.True(t, errors.As(err, &networkError))
				assert.False(t, controller.IsPermanentError(err))
				assert.Equal(t, networkErrorMsg, err.Error())
			})
		})

		when("other error", func() {
			r := &fakeErrorReconciler{err: errors.New(otherErrorMsg)}
			subject := &NetworkErrorReconciler{Reconciler: r}
			it("wraps it as permanent error", func() {
				err := subject.Reconcile(context.Background(), "whatever")

				require.Error(t, err)

				var networkError *NetworkError
				assert.False(t, errors.As(err, &networkError))
				assert.True(t, controller.IsPermanentError(err))
				assert.Equal(t, otherErrorMsg, err.Error())
			})
		})

		when("not ready error", func() {
			r := &fakeErrorReconciler{err: NewNotReadyError(otherErrorMsg)}
			subject := &NetworkErrorReconciler{Reconciler: r}
			it("returns the error", func() {
				err := subject.Reconcile(context.Background(), "whatever")

				require.Error(t, err)

				var notReadyErr *NotReadyError
				assert.True(t, errors.As(err, &notReadyErr))
				assert.False(t, controller.IsPermanentError(err))
				assert.Equal(t, otherErrorMsg, err.Error())
			})
		})
	})
}

type fakeErrorReconciler struct {
	err error
}

func (r *fakeErrorReconciler) Reconcile(ctx context.Context, key string) error {
	return r.err
}
