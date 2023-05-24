package testhelpers

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/cnb"
)

type FakeBuilderCreator struct {
	Record         buildapi.BuilderRecord
	CreateErr      error
	ObjectsToTrack []corev1.ObjectReference

	CreateBuilderCalls []CreateBuilderArgs
}

type CreateBuilderArgs struct {
	Context         context.Context
	BuilderKeychain authn.Keychain
	StackKeychain   authn.Keychain
	Fetcher         cnb.RemoteBuildpackFetcher
	ClusterStack    *buildapi.ClusterStack
	BuilderSpec     buildapi.BuilderSpec
}

func (f *FakeBuilderCreator) CreateBuilder(ctx context.Context, builderKeychain authn.Keychain, stackKeychain authn.Keychain, fetcher cnb.RemoteBuildpackFetcher, clusterStack *buildapi.ClusterStack, spec buildapi.BuilderSpec) (buildapi.BuilderRecord, error) {
	f.CreateBuilderCalls = append(f.CreateBuilderCalls, CreateBuilderArgs{
		Context:         ctx,
		BuilderKeychain: builderKeychain,
		StackKeychain:   stackKeychain,
		Fetcher:         fetcher,
		ClusterStack:    clusterStack,
		BuilderSpec:     spec,
	})

	return f.Record, f.CreateErr
}
