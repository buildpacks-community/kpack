package notary

import (
	"encoding/hex"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/buildpacks/lifecycle/platform"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	"github.com/theupdateframework/notary"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/cryptoservice"
	"github.com/theupdateframework/notary/storage"
	"github.com/theupdateframework/notary/trustmanager"
	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/signed"
)

type ImageFetcher interface {
	Fetch(keychain authn.Keychain, repoName string) (v1.Image, string, error)
}

type RepositoryFactory interface {
	GetRepository(url string, gun data.GUN, remoteStore storage.RemoteStore, cryptoService signed.CryptoService) (Repository, error)
}

type Repository interface {
	PublishTarget(target *client.Target) error
}

type ImageSigner struct {
	Logger  *log.Logger
	Client  ImageFetcher
	Factory RepositoryFactory
}

func (s *ImageSigner) Sign(url, notarySecretDir string, report platform.ExportReport, keychain authn.Keychain) error {
	gun, targets, err := s.makeGUNAndTargets(report, keychain)
	if err != nil {
		return err
	}

	remoteStore, err := storage.NewNotaryServerStore(
		url,
		gun,
		&AuthenticatingRoundTripper{
			Keychain:            keychain,
			WrappedRoundTripper: http.DefaultTransport,
		},
	)
	if err != nil {
		return err
	}

	cryptoService, err := s.makeCryptoService(notarySecretDir)
	if err != nil {
		return err
	}

	for _, target := range targets {
		repo, err := s.Factory.GetRepository(url, gun, remoteStore, cryptoService)
		if err != nil {
			return err
		}

		err = repo.PublishTarget(target)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *ImageSigner) makeGUNAndTargets(report platform.ExportReport, keychain authn.Keychain) (data.GUN, []*client.Target, error) {
	gun := data.GUN("")
	var targets []*client.Target
	for _, tag := range report.Image.Tags {
		s.Logger.Printf("Signing tag '%s'\n", tag)
		ref, err := name.ParseReference(tag, name.WeakValidation)
		if err != nil {
			return "", nil, err
		}

		s.Logger.Printf("Pulling image '%s'\n", ref.Context().Name()+"@"+report.Image.Digest)
		image, _, err := s.Client.Fetch(keychain, ref.Context().Name()+"@"+report.Image.Digest)
		if err != nil {
			return "", nil, err
		}

		imageSize, err := image.Size()
		if err != nil {
			return "", nil, err
		}

		digestBytes, err := hex.DecodeString(strings.TrimPrefix(report.Image.Digest, "sha256:"))
		if err != nil {
			return "", nil, err
		}

		curGUN := data.GUN(ref.Context().Name())
		if gun == "" {
			gun = curGUN
		} else if gun != curGUN {
			return "", nil, errors.New("signing to multiple registries is not supported")
		}

		targets = append(targets, &client.Target{
			Name: ref.Identifier(),
			Hashes: map[string][]byte{
				notary.SHA256: digestBytes,
			},
			Length: imageSize,
		})
	}

	return gun, targets, nil
}

func (s *ImageSigner) makeCryptoService(notarySecretDir string) (*cryptoservice.CryptoService, error) {
	cryptoStore := storage.NewMemoryStore(nil)

	fileInfos, err := ioutil.ReadDir(notarySecretDir)
	if err != nil {
		return nil, err
	}

	privateKeyFound := false
	for _, info := range fileInfos {
		if strings.HasSuffix(info.Name(), ".key") {
			buf, err := ioutil.ReadFile(filepath.Join(notarySecretDir, info.Name()))
			if err != nil {
				return nil, err
			}

			err = cryptoStore.Set(strings.TrimSuffix(info.Name(), ".key"), buf)
			if err != nil {
				return nil, err
			}

			s.Logger.Printf("Using private key '%s'\n", info.Name())
			privateKeyFound = true
			break
		}
	}

	if !privateKeyFound {
		return nil, errors.New("failed to find private key")
	}

	keyStore := trustmanager.NewGenericKeyStore(cryptoStore, k8sSecretPassRetriever(notarySecretDir))
	return cryptoservice.NewCryptoService(keyStore), nil
}

func k8sSecretPassRetriever(notarySecretDir string) func(_, _ string, _ bool, _ int) (passphrase string, giveup bool, err error) {
	return func(_, _ string, _ bool, _ int) (passphrase string, giveup bool, err error) {
		buf, err := ioutil.ReadFile(filepath.Join(notarySecretDir, "password"))
		return strings.TrimSpace(string(buf)), false, err
	}
}
