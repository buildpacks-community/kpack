package testhelpers

import (
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
)

type FakeBuildpackRepository struct {
	Store *expv1alpha1.Store
}

func (f FakeBuildpackRepository) FindByIdAndVersion(id, version string) (cnb.RemoteBuildpackInfo, error) {
	return cnb.RemoteBuildpackInfo{}, nil
}
