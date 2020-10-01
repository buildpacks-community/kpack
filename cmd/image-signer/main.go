package main

import (
	"crypto/tls"
	"encoding/hex"
	"flag"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/theupdateframework/notary"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/cryptoservice"
	"github.com/theupdateframework/notary/storage"
	"github.com/theupdateframework/notary/trustmanager"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf/data"
)

var (
	image     string
	imageSize int64
	host      string
)

func init() {
	flag.StringVar(&image, "image", "", "")
	flag.Int64Var(&imageSize, "image-size", 0, "")
	flag.StringVar(&host, "host", "", "")
}

func main() {
	flag.Parse()

	ref, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		log.Fatal(err)
	}

	digestBytes, err := hex.DecodeString(strings.TrimPrefix(ref.Identifier(), "sha256:"))
	if err != nil {
		log.Fatal(err)
	}

	target := &client.Target{
		Name: ref.Context().Tag("latest").Name(),
		Hashes: map[string][]byte{
			notary.SHA256: digestBytes,
		},
		Length: imageSize,
	}

	gun := data.GUN(ref.Context().Name())

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	store, err := storage.NewNotaryServerStore(host, gun, tr)
	if err != nil {
		log.Fatal(err)
	}

	memStore := storage.NewMemoryStore(nil) // TODO : seed from secret volumes
	cryptoService := cryptoservice.NewCryptoService(trustmanager.NewGenericKeyStore(memStore, noOpPassRetriever))

	repo, err := client.NewRepository(
		gun,
		host,
		store,
		store,
		trustpinning.TrustPinConfig{},
		cryptoService,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	_, err = repo.ListTargets()
	switch err.(type) {
	case client.ErrRepoNotInitialized, client.ErrRepositoryNotExist:
		keys := repo.GetCryptoService().ListKeys(data.CanonicalRootRole)

		var rootKeyID string
		if len(keys) > 0 {
			sort.Strings(keys)
			rootKeyID = keys[0]
		} else {
			rootPublicKey, err := repo.GetCryptoService().Create(data.CanonicalRootRole, "", data.ECDSAKey)
			if err != nil {
				log.Fatal(err)
			}
			rootKeyID = rootPublicKey.ID()
		}

		if err := repo.Initialize([]string{rootKeyID}, data.CanonicalSnapshotRole); err != nil {
			log.Fatal(err)
		}

		err = repo.AddTarget(target, data.CanonicalTargetsRole)
	case nil:
		// TODO : repo already exists
	default:
		log.Fatal(err)
	}

	if err == nil {
		if err := repo.Publish(); err != nil {
			log.Fatal(err)
		}
	}
}

func noOpPassRetriever(keyName, alias string, createNew bool, attempts int) (passphrase string, giveup bool, err error) {
	return "", false, nil
}
