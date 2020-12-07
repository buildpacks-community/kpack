package notary

import (
	"io/ioutil"
	"path"

	"github.com/pkg/errors"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/client/changelist"
	"github.com/theupdateframework/notary/storage"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/signed"
)

type RemoteRepositoryFactory struct {
}

func (r *RemoteRepositoryFactory) GetRepository(url string, gun data.GUN, remoteStore storage.RemoteStore, cryptoService signed.CryptoService) (Repository, error) {
	changeListDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}

	changeList, err := changelist.NewFileChangelist(changeListDir)
	if err != nil {
		return nil, err
	}

	repo, err := client.NewRepository(
		gun,
		url,
		remoteStore,
		storage.NewMemoryStore(nil),
		trustpinning.TrustPinConfig{},
		cryptoService,
		changeList,
	)
	if err != nil {
		return nil, err
	}

	return &RemoteRepository{repo: repo}, nil
}

type RemoteRepository struct {
	repo client.Repository
}

func (r *RemoteRepository) PublishTarget(target *client.Target) error {
	roles, err := r.getRoles(target)
	if err != nil {
		return err
	}

	err = r.repo.AddTarget(target, roles...)
	if err != nil {
		return err

	}

	return r.repo.Publish()
}

func (r *RemoteRepository) getRoles(target *client.Target) ([]data.RoleName, error) {
	delegationRoles, err := r.repo.GetDelegationRoles()
	if err != nil {
		return nil, err
	}

	if len(delegationRoles) == 0 {
		return []data.RoleName{data.CanonicalTargetsRole}, nil
	}

	keys := r.repo.GetCryptoService().ListAllKeys()
	if len(keys) != 1 {
		return nil, errors.Errorf("expected exactly one signing key but got %d", len(keys))
	}

	var roles []data.RoleName
	for _, delegationRole := range delegationRoles {
		if path.Dir(delegationRole.Name.String()) != data.CanonicalTargetsRole.String() || !delegationRole.CheckPaths(target.Name) {
			continue
		}

		for _, keyID := range delegationRole.KeyIDs {
			if _, ok := keys[keyID]; ok {
				roles = append(roles, delegationRole.Name)
				break
			}
		}
	}

	if len(roles) == 0 {
		return []data.RoleName{}, errors.New("no delegation roles found")
	}

	return roles, nil
}
