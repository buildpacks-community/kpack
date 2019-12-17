package testhelpers

import (
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
)

type FakeBuilderCreator struct {
	Record    v1alpha1.BuilderRecord
	CreateErr error

	CreateBuilderCalls []CreateBuilderArgs
}

type CreateBuilderArgs struct {
	Keychain            authn.Keychain
	BuildpackRepository cnb.BuildpackRepository
	CustomBuilderSpec   expv1alpha1.CustomBuilderSpec
}

func (f *FakeBuilderCreator) CreateBuilder(keychain authn.Keychain, repo cnb.BuildpackRepository, builder expv1alpha1.CustomBuilderSpec) (v1alpha1.BuilderRecord, error) {
	f.CreateBuilderCalls = append(f.CreateBuilderCalls, CreateBuilderArgs{
		Keychain:            keychain,
		BuildpackRepository: repo,
		CustomBuilderSpec:   builder,
	})

	return f.Record, f.CreateErr
}
