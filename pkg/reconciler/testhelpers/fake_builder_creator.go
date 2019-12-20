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
}

func (f *FakeBuilderCreator) CreateBuilder(authn.Keychain, cnb.Store, expv1alpha1.CustomBuilderSpec) (v1alpha1.BuilderRecord, error) {
	return f.Record, f.CreateErr
}
