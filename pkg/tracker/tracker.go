/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// modified from https://knative.dev/pkg/tree/master/tracker
// The version provided by knative/pkg forces tracking on namespace scoped
// object an in our case the ClusterBuilder is a cluster scoped
// object that need to be tracked

package tracker

import (
	"sync"
	"time"

	"github.com/pivotal/kpack/pkg/reconciler"
	"k8s.io/apimachinery/pkg/types"
)

func New(callback func(types.NamespacedName), lease time.Duration) *Tracker {
	return &Tracker{
		leaseDuration: lease,
		cb:            callback,
	}
}

type Tracker struct {
	m sync.Mutex
	// mapping maps from an object reference to the set of
	// keys for objects watching it.
	mapping map[string]set

	// The amount of time that an object may watch another
	// before having to renew the lease.
	leaseDuration time.Duration

	cb func(types.NamespacedName)
}

// set is a map from keys to expirations
type set map[types.NamespacedName]time.Time

// Track implements Interface.
func (i *Tracker) Track(ref reconciler.Key, obj types.NamespacedName) error {
	i.m.Lock()
	defer i.m.Unlock()
	if i.mapping == nil {
		i.mapping = make(map[string]set)
	}

	l, ok := i.mapping[ref.String()]
	if !ok {
		l = set{}
	}
	// Overwrite the key with a new expiration.
	l[obj] = time.Now().Add(i.leaseDuration)

	i.mapping[ref.String()] = l
	return nil
}

func isExpired(expiry time.Time) bool {
	return time.Now().After(expiry)
}

// OnChanged implements Interface.
func (i *Tracker) OnChanged(obj interface{}) {
	reconcilerObj, ok := obj.(reconciler.Object)
	if !ok {
		return
	}

	key := reconciler.KeyForObject(reconcilerObj)

	// TODO(mattmoor): Consider locking the mapping (global) for a
	// smaller scope and leveraging a per-set lock to guard its access.
	i.m.Lock()
	defer i.m.Unlock()
	s, ok := i.mapping[key.String()]
	if !ok {
		// TODO(mattmoor): We should consider logging here.
		return
	}

	for key, expiry := range s {
		// If the expiration has lapsed, then delete the key.
		if isExpired(expiry) {
			delete(s, key)
			continue
		}
		i.cb(key)
	}

	if len(s) == 0 {
		delete(i.mapping, key.String())
	}
}
