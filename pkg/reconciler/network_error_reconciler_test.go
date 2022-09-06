package reconciler

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
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
			subject := &NetworkErrorReconciler{Reconciler: &fakeErrorReconciler{retry: true}}
			it("re-throw the error", func() {
				err := subject.Reconcile(context.Background(), "whatever")

				require.Error(t, err)

				var networkError *NetworkError
				require.True(t, errors.As(err, &networkError))
				require.Equal(t, networkErrorMsg, err.Error())
			})
		})

		when("other error", func() {
			subject := &NetworkErrorReconciler{Reconciler: &fakeErrorReconciler{retry: false}}
			it("wraps it as permanent error", func() {
				err := subject.Reconcile(context.Background(), "whatever")

				require.Error(t, err)

				var networkError *NetworkError
				require.False(t, errors.As(err, &networkError))
				require.Equal(t, otherErrorMsg, err.Error())
			})
		})
	})
}

type fakeErrorReconciler struct {
	retry bool
}

func (r *fakeErrorReconciler) Reconcile(ctx context.Context, key string) error {
	if r.retry {
		err := errors.New(networkErrorMsg)
		return &NetworkError{Err: err}
	}
	err := errors.New(otherErrorMsg)
	return err
}
