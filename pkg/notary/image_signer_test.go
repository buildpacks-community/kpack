package notary

import (
	"encoding/hex"
	"io/ioutil"
	"log"
	"path/filepath"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	"github.com/theupdateframework/notary"
	notaryclient "github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/storage"
	"github.com/theupdateframework/notary/tuf/data"
	"github.com/theupdateframework/notary/tuf/signed"

	"github.com/pivotal/kpack/pkg/registry/registryfakes"
)

func TestImageSigner(t *testing.T) {
	spec.Run(t, "Test Image Signer", testImageSigner)
}

func testImageSigner(t *testing.T, when spec.G, it spec.S) {
	var (
		logger = log.New(ioutil.Discard, "", 0)

		client = registryfakes.NewFakeClient()

		factory = &FakeRepositoryFactory{}

		signer = ImageSigner{
			Logger:  logger,
			Client:  client,
			Factory: factory,
		}
	)

	when("#Sign", func() {
		var (
			keychain authn.Keychain
			image    v1.Image
		)

		it.Before(func() {
			keychain = &registryfakes.FakeKeychain{}

			var err error
			image, err = random.Image(0, 0)
			require.NoError(t, err)

			client.AddImage("example-registry.io/test@sha256:a15790640a6690aa1730c38cf0a440e2aa44aaca9b0e8931a9f2b0d7cc90fd65", image, keychain)
			client.AddImage("some-other-registry.io/test@sha256:a15790640a6690aa1730c38cf0a440e2aa44aaca9b0e8931a9f2b0d7cc90fd65", image, keychain)
		})

		it("signs images", func() {
			notaryDir := filepath.Join("testdata", "notary")
			reportPath := filepath.Join("testdata", "report.toml")

			err := signer.Sign("https://example.com/notary", notaryDir, reportPath, "", keychain)
			require.NoError(t, err)

			require.Len(t, factory.Calls, 2)
			for i := range factory.Calls {
				require.Equal(t, "https://example.com/notary", factory.Calls[i].URL)
				require.Equal(t, data.GUN("example-registry.io/test"), factory.Calls[i].GUN)
			}

			require.Len(t, factory.FakeRepository.PublishedTargets, 2)
			require.Equal(t, "latest", factory.FakeRepository.PublishedTargets[0].Name)
			require.Equal(t, "00000000  a1 57 90 64 0a 66 90 aa  17 30 c3 8c f0 a4 40 e2  |.W.d.f...0....@.|\n00000010  aa 44 aa ca 9b 0e 89 31  a9 f2 b0 d7 cc 90 fd 65  |.D.....1.......e|\n", hex.Dump(factory.FakeRepository.PublishedTargets[0].Hashes[notary.SHA256]))
			require.Equal(t, int64(264), factory.FakeRepository.PublishedTargets[0].Length)

			require.Equal(t, "other-tag", factory.FakeRepository.PublishedTargets[1].Name)
		})

		it("validates the GUN is uniform for all tags", func() {
			notaryDir := filepath.Join("testdata", "notary")
			reportPath := filepath.Join("testdata", "report-multiple-gun.toml")

			err := signer.Sign("https://example.com/notary", notaryDir, reportPath, "", keychain)
			require.EqualError(t, err, "signing to multiple registries is not supported")
		})

		it("validates the notary private key exists", func() {
			notaryDir := filepath.Join("testdata", "notary-no-key")
			reportPath := filepath.Join("testdata", "report.toml")

			err := signer.Sign("https://example.com/notary", notaryDir, reportPath, "", keychain)
			require.EqualError(t, err, "failed to find private key")
		})
	})
}

type FakeRepositoryFactoryCall struct {
	URL string
	GUN data.GUN
}

type FakeRepositoryFactory struct {
	Calls          []FakeRepositoryFactoryCall
	FakeRepository *FakeRepository
}

func (f *FakeRepositoryFactory) GetRepository(url string, gun data.GUN, _ storage.RemoteStore, _ signed.CryptoService) (Repository, error) {
	if f.FakeRepository == nil {
		f.FakeRepository = &FakeRepository{}
	}
	f.Calls = append(f.Calls, FakeRepositoryFactoryCall{
		URL: url,
		GUN: gun,
	})
	return f.FakeRepository, nil
}

type FakeRepository struct {
	PublishedTargets []*notaryclient.Target
}

func (f *FakeRepository) PublishTarget(target *notaryclient.Target) error {
	f.PublishedTargets = append(f.PublishedTargets, target)
	return nil
}
