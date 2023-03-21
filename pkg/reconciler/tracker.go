package reconciler

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type Key struct {
	GroupKind      schema.GroupKind
	NamespacedName types.NamespacedName
}

func (k Key) String() string {
	return fmt.Sprintf("%s/%s", k.GroupKind, k.NamespacedName)
}

func (k Key) WithNamespace(namespace string) Key {
	return Key{
		GroupKind: k.GroupKind,
		NamespacedName: types.NamespacedName{
			Namespace: namespace,
			Name:      k.NamespacedName.Name,
		},
	}
}

type Object interface {
	GetName() string
	GetNamespace() string
	GetObjectKind() schema.ObjectKind
}

func KeyForObject(obj Object) Key {
	return Key{
		NamespacedName: types.NamespacedName{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
		GroupKind: obj.GetObjectKind().GroupVersionKind().GroupKind(),
	}
}

type Tracker interface {
	Track(ref Key, obj types.NamespacedName)
	TrackKind(kind schema.GroupKind, obj types.NamespacedName)
	OnChanged(obj interface{})
}
