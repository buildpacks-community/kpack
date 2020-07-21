package testhelpers

import (
	v1alpha1 "github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
)

type FakeBuildpackRepository struct {
	ClusterStore *v1alpha1.ClusterStore
}

func (f FakeBuildpackRepository) FindByIdAndVersion(id, version string) (cnb.RemoteBuildpackInfo, error) {
	return cnb.RemoteBuildpackInfo{}, nil
}
