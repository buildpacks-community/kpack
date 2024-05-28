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
	Context            context.Context
	BuilderKeychain    authn.Keychain
	StackKeychain      authn.Keychain
	Fetcher            cnb.RemoteBuildpackFetcher
	ClusterStack       *buildapi.ClusterStack
	ClusterLifecycle   *buildapi.ClusterLifecycle
	BuilderSpec        buildapi.BuilderSpec
	SigningSecrets     []*corev1.Secret
	ResolvedBuilderTag string
}

func (f *FakeBuilderCreator) CreateBuilder(
	ctx context.Context,
	builderKeychain authn.Keychain,
	stackKeychain authn.Keychain,
	fetcher cnb.RemoteBuildpackFetcher,
	clusterStack *buildapi.ClusterStack,
	clusterLifecycle *buildapi.ClusterLifecycle,
	spec buildapi.BuilderSpec,
	signingSecrets []*corev1.Secret,
	resolvedBuilderTag string,
) (buildapi.BuilderRecord, error) {
	f.CreateBuilderCalls = append(f.CreateBuilderCalls, CreateBuilderArgs{
		Context:            ctx,
		BuilderKeychain:    builderKeychain,
		StackKeychain:      stackKeychain,
		Fetcher:            fetcher,
		ClusterStack:       clusterStack,
		ClusterLifecycle:   clusterLifecycle,
		BuilderSpec:        spec,
		SigningSecrets:     signingSecrets,
		ResolvedBuilderTag: resolvedBuilderTag,
	})

	return f.Record, f.CreateErr
}
