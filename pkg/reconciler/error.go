package reconciler

import "github.com/pkg/errors"

type NotReadyError struct {
	err error
}

func (e *NotReadyError) Error() string {
	return e.err.Error()
}

func NewNotReadyError(format string, args ...interface{}) error {
	return &NotReadyError{err: errors.Errorf(format, args...)}
}
