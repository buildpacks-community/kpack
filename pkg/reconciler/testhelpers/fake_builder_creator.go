package testhelpers

import (
	"github.com/google/go-containerregistry/pkg/authn"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

type FakeBuilderCreator struct {
	Record    buildapi.BuilderRecord
	CreateErr error

	CreateBuilderCalls []CreateBuilderArgs
}

type CreateBuilderArgs struct {
	Keychain     authn.Keychain
	ClusterStack *buildapi.ClusterStack
	ClusterStore *buildapi.ClusterStore
	BuilderSpec  buildapi.BuilderSpec
}

func (f *FakeBuilderCreator) CreateBuilder(keychain authn.Keychain, clusterStore *buildapi.ClusterStore, clusterStack *buildapi.ClusterStack, builder buildapi.BuilderSpec) (buildapi.BuilderRecord, error) {
	f.CreateBuilderCalls = append(f.CreateBuilderCalls, CreateBuilderArgs{
		Keychain:     keychain,
		ClusterStore: clusterStore,
		ClusterStack: clusterStack,
		BuilderSpec:  builder,
	})

	return f.Record, f.CreateErr
}

func (f *FakeBuilderCreator) WithSecrets(secrets []corev1.Secret) {
}
