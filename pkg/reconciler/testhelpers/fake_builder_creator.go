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
	Context      context.Context
	Keychain     authn.Keychain
	Fetcher      cnb.RemoteBuildpackFetcher
	ClusterStack *buildapi.ClusterStack
	BuilderSpec  buildapi.BuilderSpec
}

func (f *FakeBuilderCreator) CreateBuilder(ctx context.Context, keychain authn.Keychain, fetcher cnb.RemoteBuildpackFetcher, clusterStack *buildapi.ClusterStack, builder buildapi.BuilderSpec) ([]corev1.ObjectReference, buildapi.BuilderRecord, error) {
	f.CreateBuilderCalls = append(f.CreateBuilderCalls, CreateBuilderArgs{
		Context:      ctx,
		Keychain:     keychain,
		Fetcher:      fetcher,
		ClusterStack: clusterStack,
		BuilderSpec:  builder,
	})

	return f.ObjectsToTrack, f.Record, f.CreateErr
}
