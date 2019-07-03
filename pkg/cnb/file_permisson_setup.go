package cnb

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/pivotal/build-service-system/pkg/registry"
)

type chowner interface {
	Chown(volume string, uid, gid int) error
}

type FilePermissionSetup struct {
	RemoteImageFactory registry.RemoteImageFactory
	Chowner            chowner
}

const cnbUserId = "CNB_USER_ID"
const cnbGroupId = "CNB_GROUP_ID"

func (p *FilePermissionSetup) Setup(builder string, volumes ...string) error {
	image, err := p.RemoteImageFactory.NewRemote(registry.NewNoAuthImageRef(builder))
	if err != nil {

		return err
	}

	uid, err := parseCNBID(image, cnbUserId)
	if err != nil {
		return err
	}

	gid, err := parseCNBID(image, cnbGroupId)
	if err != nil {
		return err
	}

	for _, volume := range volumes {
		if err := p.chownR(volume, uid, gid); err != nil {
			return err
		}
	}

	return nil
}

func parseCNBID(image registry.RemoteImage, env string) (int, error) {
	v, err := image.Env(env)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(v)
}

func (p *FilePermissionSetup) chownR(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = p.Chowner.Chown(name, uid, gid)
		}
		return err
	})
}
