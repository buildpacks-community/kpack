package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"github.com/theupdateframework/notary"
	"github.com/theupdateframework/notary/client"
	"github.com/theupdateframework/notary/client/changelist"
	"github.com/theupdateframework/notary/cryptoservice"
	"github.com/theupdateframework/notary/storage"
	"github.com/theupdateframework/notary/trustmanager"
	"github.com/theupdateframework/notary/trustpinning"
	"github.com/theupdateframework/notary/tuf/data"
	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeConfig       string
	masterURL        string
	image            string
	imageSize        int64
	host             string
	rootSecretName   string
	targetSecretName string
	namespace        string
)

const NotaryV1SecretAnnotation = "v1.notary.kpack.io/id"

func init() {
	flag.StringVar(&kubeConfig, "kube-config", "", "")
	flag.StringVar(&masterURL, "master-url", "", "")
	flag.StringVar(&image, "image", "", "")
	flag.Int64Var(&imageSize, "image-size", 0, "")
	flag.StringVar(&host, "host", "", "")
	flag.StringVar(&rootSecretName, "root-secret", "", "")
	flag.StringVar(&targetSecretName, "target-secret", "", "")
	flag.StringVar(&namespace, "namespace", "", "")
}

func main() {
	flag.Parse()

	clusterConfig, err := clientcmd.BuildConfigFromFlags(masterURL, kubeConfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	k8sClient, err := kubernetes.NewForConfig(clusterConfig)
	if err != nil {
		log.Fatalf("could not get kubernetes client: %s", err)
	}

	cryptoStore := NewK8sStorage(k8sClient, rootSecretName, targetSecretName, namespace)

	err = cryptoStore.Load()
	if err != nil {
		log.Fatal(err)
	}

	ref, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		log.Fatal(err)
	}

	digestBytes, err := hex.DecodeString(strings.TrimPrefix(ref.Identifier(), "sha256:"))
	if err != nil {
		log.Fatal(err)
	}

	target := &client.Target{
		Name: "latest",
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

	remoteStore, err := storage.NewNotaryServerStore(host, gun, tr)
	if err != nil {
		log.Fatal(err)
	}

	// TODO : don't use a memory store here, should be a custom in-memory impl.
	localStore := storage.NewMemoryStore(nil)

	clDir, err := ioutil.TempDir("", "")
	if err != nil {
		log.Fatal(err)
	}

	// TODO : don't need a file based changelist, can be in-memory
	cl, err := changelist.NewFileChangelist(clDir)
	if err != nil {
		log.Fatal(err)
	}

	cryptoService := cryptoservice.NewCryptoService(trustmanager.NewGenericKeyStore(cryptoStore, noOpPassRetriever))

	repo, err := client.NewRepository(
		gun,
		host,
		remoteStore,
		localStore,
		trustpinning.TrustPinConfig{},
		cryptoService,
		cl,
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
		// FIXME : does not handle delegates
		err = repo.AddTarget(target, data.CanonicalTargetsRole)
	default:
		log.Fatal(err)
	}

	if err == nil {
		if err := repo.Publish(); err != nil {
			log.Fatal(err)
		}
	}

	err = cryptoStore.Save()
	if err != nil {
		log.Fatal(err)
	}
}

func noOpPassRetriever(_, _ string, _ bool, _ int) (passphrase string, giveup bool, err error) {
	return "", false, nil
}

type K8sStorage struct {
	K8sClient        kubernetes.Interface
	RootSecretName   string
	TargetSecretName string
	Namespace        string
	Secrets          map[string]*v1.Secret
}

func NewK8sStorage(client kubernetes.Interface, rootSecretName, targetSecretName, namespace string) *K8sStorage {
	return &K8sStorage{
		K8sClient:        client,
		RootSecretName:   rootSecretName,
		TargetSecretName: targetSecretName,
		Namespace:        namespace,
		Secrets:          map[string]*v1.Secret{},
	}
}

func (k *K8sStorage) Load() error {
	for _,  secretName := range []string{k.RootSecretName, k.TargetSecretName} {
		secret, err := k.K8sClient.CoreV1().Secrets(namespace).Get(secretName, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		k.Secrets[secret.Annotations[NotaryV1SecretAnnotation]] = secret
	}
	return nil
}

func (k *K8sStorage) Save() error {
	for _, secret := range k.Secrets {
		_, err := k.K8sClient.CoreV1().Secrets(secret.Namespace).Get(secret.Name, metav1.GetOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}

		if k8serrors.IsNotFound(err) {
			_, err = k.K8sClient.CoreV1().Secrets(secret.Namespace).Create(secret)
		} else {
			_, err = k.K8sClient.CoreV1().Secrets(secret.Namespace).Update(secret)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *K8sStorage) Set(fileName string, data []byte) error {
	var secretName string

	scanner := bufio.NewScanner(bytes.NewBuffer(data))
	for scanner.Scan() {
		text := scanner.Text()
		if strings.Contains(text, "role") {
			split := strings.Split(text, ":")
			if len(split) != 2 {
				return errors.New("unable to get secret role")
			}

			secretType := strings.TrimSpace(split[1])
			switch secretType {
			case "root":
				secretName = k.RootSecretName
			case "targets":
				secretName = k.TargetSecretName
			default:
				return errors.Errorf("unknown secret type %s", secretType)
			}
			break
		}
	}

	if secretName == "" {
		return errors.New("unknown secret")
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: k.Namespace,
			Annotations: map[string]string{
				NotaryV1SecretAnnotation: fileName,
			},
		},
		Data: map[string][]byte{
			"cert": data,
		},
	}
	k.Secrets[fileName] = secret

	return nil
}

func (k *K8sStorage) Remove(fileName string) error {
	delete(k.Secrets, fileName)
	return nil
}

func (k *K8sStorage) Get(fileName string) ([]byte, error) {
	secret, ok := k.Secrets[fileName]
	if ok {
		return secret.Data["cert"], nil
	}
	return nil, errors.Errorf("failed to find %s", fileName)
}

func (k *K8sStorage) ListFiles() []string {
	var files []string
	for f := range k.Secrets {
		files = append(files, f)
	}
	return files
}

func (k *K8sStorage) Location() string {
	return "k8s"
}
