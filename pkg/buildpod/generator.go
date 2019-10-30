package buildpod

import (
	"encoding/json"
	"strconv"

	"github.com/buildpack/lifecycle/metadata"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/registry"
)

type Generator struct {
	BuildPodConfig     v1alpha1.BuildPodImages
	K8sClient          k8sclient.Interface
	RemoteImageFactory registry.RemoteImageFactory
}

func (g *Generator) Generate(build *v1alpha1.Build) (*v1.Pod, error) {
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

func (g *Generator) fetchBuildSecrets(build *v1alpha1.Build) ([]corev1.Secret, error) {
	var secrets []corev1.Secret
	serviceAccount, err := g.K8sClient.CoreV1().ServiceAccounts(build.Namespace).Get(build.Spec.ServiceAccount, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, secretRef := range serviceAccount.Secrets {
		secret, err := g.K8sClient.CoreV1().Secrets(build.Namespace).Get(secretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		secrets = append(secrets, *secret)
	}
	return secrets, nil
}

const cnbUserId = "CNB_USER_ID"
const cnbGroupId = "CNB_GROUP_ID"

func (g *Generator) fetchBuilderConfig(build *v1alpha1.Build) (v1alpha1.BuildPodBuilderConfig, error) {
	builderImage, err := g.RemoteImageFactory.NewRemote(build.Spec.Builder.Image, registry.SecretRef{
		Namespace:        build.Namespace,
		ImagePullSecrets: build.Spec.Builder.ImagePullSecrets,
	})
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, err
	}

	builderStackID, err := builderImage.Label(metadata.StackMetadataLabel)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, errors.Wrap(err, "builder image stack ID label not present")
	}

	metadataJSON, err := builderImage.Label(cnb.BuilderMetadataLabel)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, errors.Wrap(err, "builder image metadata label not present")
	}

	var builderMetadata cnb.BuilderImageMetadata
	err = json.Unmarshal([]byte(metadataJSON), &builderMetadata)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, errors.Wrap(err, "unsupported builder metadata structure")
	}

	runImage, err := resolveRunImage(build, builderMetadata)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, errors.Wrap(err, "failed to resolve run image")
	}

	// Parse the run image so that we can strip the digest. We have to do this
	// for now because exporter does not support run images with digests.
	runImageRef, err := name.ParseReference(runImage, name.WeakValidation)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, err
	}

	uid, err := parseCNBID(builderImage, cnbUserId)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, err
	}

	gid, err := parseCNBID(builderImage, cnbGroupId)
	if err != nil {
		return v1alpha1.BuildPodBuilderConfig{}, err
	}

	return v1alpha1.BuildPodBuilderConfig{
		BuilderSpec: build.Spec.Builder,
		StackID:     builderStackID,
		RunImage:    runImageRef.Context().Name(),
		Uid:         uid,
		Gid:         gid,
	}, nil
}

func resolveRunImage(build *v1alpha1.Build, builderMetadata cnb.BuilderImageMetadata) (string, error) {
	ref, err := name.ParseReference(build.Spec.Tags[0], name.WeakValidation)
	if err != nil {
		return "", err
	}

	metadataMirrors := builderMetadata.Stack.RunImage.Mirrors
	if build.Spec.Builder.RunImage == nil {
		return getBestRunImage(ref.Context().RegistryStr(), builderMetadata.Stack.RunImage.Image, metadataMirrors), nil
	}

	var mirrors []string
	for _, mirror := range build.Spec.Builder.RunImage.Mirrors {
		mirrors = append(mirrors, mirror.Image)
	}
	mirrors = append(mirrors, metadataMirrors...)
	return getBestRunImage(ref.Context().RegistryStr(), build.Spec.Builder.RunImage.Image, mirrors), nil
}

func parseCNBID(image registry.RemoteImage, env string) (int64, error) {
	v, err := image.Env(env)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

func getBestRunImage(registry string, runImage string, mirrors []string) string {
	runImageList := append([]string{runImage}, mirrors...)
	for _, img := range runImageList {
		ref, err := name.ParseReference(img, name.WeakValidation)
		if err != nil {
			continue
		}
		if ref.Context().RegistryStr() == registry {
			return img
		}
	}
	return runImage
}
