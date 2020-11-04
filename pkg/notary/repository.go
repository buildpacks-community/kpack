package notary

import (
	"io/ioutil"

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
	err := r.repo.AddTarget(target, data.CanonicalTargetsRole)
	if err != nil {
		return err

	}
	return r.repo.Publish()
}
