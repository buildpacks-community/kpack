package testhelpers

import (
	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/cnb"
)

type FakeBuildpackRepository struct {
	ClusterStore *buildapi.ClusterStore
}

func (f FakeBuildpackRepository) FindByIdAndVersion(id, version string) (cnb.RemoteBuildpackInfo, error) {
	return cnb.RemoteBuildpackInfo{}, nil
}
