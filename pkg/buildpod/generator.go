package buildpod

import (
	"strconv"

	"github.com/buildpack/lifecycle/metadata"
	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const (
	cnbUserId  = "CNB_USER_ID"
	cnbGroupId = "CNB_GROUP_ID"
)

type ImageFetcher interface {
	Fetch(keychain authn.Keychain, repoName string) (ggcrv1.Image, string, error)
}

type Generator struct {
	BuildPodConfig  v1alpha1.BuildPodImages
	K8sClient       k8sclient.Interface
	KeychainFactory registry.KeychainFactory
	ImageFetcher    ImageFetcher
}

type BuildPodable interface {
	GetName() string
	GetNamespace() string
	ServiceAccount() string
	BuilderSpec() v1alpha1.BuildBuilderSpec

	BuildPod(v1alpha1.BuildPodImages, []corev1.Secret, v1alpha1.BuildPodBuilderConfig) (*corev1.Pod, error)
}

func (g *Generator) Generate(build BuildPodable) (*v1.Pod, error) {
	secrets, err := g.fetchBuildSecrets(build)
	if err != nil {
		return nil, err
	}

	buildPodBuilderConfig, err := g.fetchBuilderConfig(build)
	if err != nil {
		return nil, err
	}

	return build.BuildPod(g.BuildPodConfig, secrets, buildPodBuilderConfig)
}

func (g *Generator) fetchBuildSecrets(build BuildPodable) ([]corev1.Secret, error) {
	var secrets []corev1.Secret
	serviceAccount, err := g.K8sClient.CoreV1().ServiceAccounts(build.GetNamespace()).Get(build.ServiceAccount(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, secretRef := range serviceAccount.Secrets {
		secret, err := g.K8sClient.CoreV1().Secrets(build.GetNamespace()).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, *secret)
	}
	return secrets, nil
}

func (g *Generator) fetchBuilderConfig(build BuildPodable) (v1alpha1.BuildPodBuilderConfig, error) {
	keychain, err := g.KeychainFactory.KeychainForSecretRef(registry.SecretRef{
		Namespace:        build.GetNamespace(),
		ImagePullSecrets: build.BuilderSpec().ImagePullSecrets,
		ServiceAccount:   build.ServiceAccount(),
	})
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, errors.Wrap(err, "unable to create builder image keychain")
	}

	image, _, err := g.ImageFetcher.Fetch(keychain, build.BuilderSpec().Image)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, errors.Wrap(err, "unable to fetch remote builder image")
	}

	stackId, err := imagehelpers.GetStringLabel(image, metadata.StackMetadataLabel)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, errors.Wrap(err, "builder image stack ID label not present")
	}

	var metadata cnb.BuilderImageMetadata
	err = imagehelpers.GetLabel(image, cnb.BuilderMetadataLabel, &metadata)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, errors.Wrap(err, "unable to get builder metadata")
	}

	uid, err := parseCNBID(image, cnbUserId)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, err
	}

	gid, err := parseCNBID(image, cnbGroupId)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, err
	}

	return v1alpha1.BuildPodBuilderConfig{
		StackID:     stackId,
		RunImage:    metadata.Stack.RunImage.Image,
		PlatformAPI: metadata.Lifecycle.API.PlatformVersion,
		Uid:         uid,
		Gid:         gid,
	}, nil
}

func parseCNBID(image ggcrv1.Image, env string) (int64, error) {
	v, err := imagehelpers.GetEnv(image, env)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}
