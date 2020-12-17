package testhelpers

import (
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
)

type FakeBuilderCreator struct {
	Record    v1alpha1.BuilderRecord
	CreateErr error

	CreateBuilderCalls []CreateBuilderArgs
}

type CreateBuilderArgs struct {
	Keychain     authn.Keychain
	ClusterStack *v1alpha1.ClusterStack
	ClusterStore *v1alpha1.ClusterStore
	BuilderSpec  v1alpha1.BuilderSpec
}

func (f *FakeBuilderCreator) CreateBuilder(keychain authn.Keychain, clusterStore *v1alpha1.ClusterStore, clusterStack *v1alpha1.ClusterStack, builder v1alpha1.BuilderSpec) (v1alpha1.BuilderRecord, error) {
	f.CreateBuilderCalls = append(f.CreateBuilderCalls, CreateBuilderArgs{
		Keychain:     keychain,
		ClusterStore: clusterStore,
		ClusterStack: clusterStack,
		BuilderSpec:  builder,
	})

	return f.Record, f.CreateErr
}
