package testhelpers

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/cnb"
)

type FakeBuilderCreator struct {
	Record    buildapi.BuilderRecord
	CreateErr error

	CreateBuilderCalls []CreateBuilderArgs
}

type CreateBuilderArgs struct {
	Context      context.Context
	Keychain     authn.Keychain
	Fetcher      cnb.RemoteBuildpackFetcher
	ClusterStack *buildapi.ClusterStack
	BuilderSpec  buildapi.BuilderSpec
}

var _ cnb.BuilderCreator = (*FakeBuilderCreator)(nil)

func (f *FakeBuilderCreator) CreateBuilder(ctx context.Context, keychain authn.Keychain, fetcher cnb.RemoteBuildpackFetcher, clusterStack *buildapi.ClusterStack, builder buildapi.BuilderSpec) (buildapi.BuilderRecord, error) {
	f.CreateBuilderCalls = append(f.CreateBuilderCalls, CreateBuilderArgs{
		Context:      ctx,
		Keychain:     keychain,
		Fetcher:      fetcher,
		ClusterStack: clusterStack,
		BuilderSpec:  builder,
	})

	return f.Record, f.CreateErr
}
