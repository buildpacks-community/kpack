package testhelpers

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"
)

type CallCounter struct {
	actions []testing.Action
}

func (c *CallCounter) Reactor(action testing.Action) (handled bool, ret runtime.Object, err error) {
	c.actions = append(c.actions, action)
	return false, nil, nil
}

func (c *CallCounter) UpdateCalls() int {
	calls := 0
	for _, a := range c.actions {
		if a.GetVerb() == "update" {
			calls++
		}

	}
	return calls
}
