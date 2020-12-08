package buildpod

import (
	"fmt"
	"strconv"

	"github.com/buildpacks/lifecycle"
	"github.com/google/go-containerregistry/pkg/authn"
	ggcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	k8sclient "k8s.io/client-go/kubernetes"
	"knative.dev/pkg/apis/duck"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/duckprovisionedserviceable"
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
	BuildPodConfig  v1alpha2.BuildPodImages
	K8sClient       k8sclient.Interface
	DynamicClient   dynamic.Interface
	KeychainFactory registry.KeychainFactory
	ImageFetcher    ImageFetcher
}

type BuildPodable interface {
	GetName() string
	GetNamespace() string
	ServiceAccount() string
	BuilderSpec() v1alpha1.BuildBuilderSpec
	Services() v1alpha2.Services
	V1Alpha1Bindings() (v1alpha1.Bindings, error)

	BuildPod(v1alpha2.BuildPodImages, []corev1.Secret, v1alpha2.BuildPodBuilderConfig, []v1alpha2.ServiceBinding) (*corev1.Pod, error)
}

func (g *Generator) Generate(build BuildPodable) (*v1.Pod, error) {
	serviceBindings, err := g.fetchServiceBindings(build)
	if err != nil {
		return nil, err
	}

	if err := g.buildAllowed(build, serviceBindings); err != nil {
		return nil, fmt.Errorf("build rejected: %w", err)
	}

	secrets, err := g.fetchBuildSecrets(build)
	if err != nil {
		return nil, err
	}

	buildPodBuilderConfig, err := g.fetchBuilderConfig(build)
	if err != nil {
		return nil, err
	}

	return build.BuildPod(g.BuildPodConfig, secrets, buildPodBuilderConfig, serviceBindings)
}

func (g *Generator) buildAllowed(build BuildPodable, serviceBindings []v1alpha2.ServiceBinding) error {
	serviceAccounts, err := g.fetchServiceAccounts(build)
	if err != nil {
		return err
	}

	var forbiddenSecrets = map[string]bool{}
	for _, serviceAccount := range serviceAccounts {
		for _, secret := range serviceAccount.Secrets {
			forbiddenSecrets[secret.Name] = true
		}
	}

	for _, sb := range serviceBindings {
		if forbiddenSecrets[sb.SecretRef.Name] {
			return fmt.Errorf("service %q uses forbidden secret %q", sb.Name, sb.SecretRef.Name)
		}
	}

	return nil
}

func (g *Generator) fetchServiceAccounts(build BuildPodable) ([]corev1.ServiceAccount, error) {
	serviceAccounts, err := g.K8sClient.CoreV1().ServiceAccounts(build.GetNamespace()).List(metav1.ListOptions{})
	if err != nil {
		return []v1.ServiceAccount{}, err
	}
	return serviceAccounts.Items, nil
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

func (g *Generator) fetchBuilderConfig(build BuildPodable) (v1alpha2.BuildPodBuilderConfig, error) {
	keychain, err := g.KeychainFactory.KeychainForSecretRef(registry.SecretRef{
		Namespace:        build.GetNamespace(),
		ImagePullSecrets: build.BuilderSpec().ImagePullSecrets,
		ServiceAccount:   build.ServiceAccount(),
	})
	if err != nil {
		return v1alpha2.BuildPodBuilderConfig{}, errors.Wrap(err, "unable to create builder image keychain")
	}

	image, _, err := g.ImageFetcher.Fetch(keychain, build.BuilderSpec().Image)
	if err != nil {
		return v1alpha2.BuildPodBuilderConfig{}, errors.Wrap(err, "unable to fetch remote builder image")
	}

	stackId, err := imagehelpers.GetStringLabel(image, lifecycle.StackIDLabel)
	if err != nil {
		return v1alpha2.BuildPodBuilderConfig{}, errors.Wrap(err, "builder image stack ID label not present")
	}

	var metadata cnb.BuilderImageMetadata
	err = imagehelpers.GetLabel(image, cnb.BuilderMetadataLabel, &metadata)
	if err != nil {
		return v1alpha2.BuildPodBuilderConfig{}, errors.Wrap(err, "unable to get builder metadata")
	}

	uid, err := parseCNBID(image, cnbUserId)
	if err != nil {
		return v1alpha2.BuildPodBuilderConfig{}, err
	}

	gid, err := parseCNBID(image, cnbGroupId)
	if err != nil {
		return v1alpha2.BuildPodBuilderConfig{}, err
	}

	return v1alpha2.BuildPodBuilderConfig{
		StackID:     stackId,
		RunImage:    metadata.Stack.RunImage.Image,
		PlatformAPI: metadata.Lifecycle.API.PlatformVersion,
		Uid:         uid,
		Gid:         gid,
	}, nil
}

func (g *Generator) fetchServiceBindings(build BuildPodable) ([]v1alpha2.ServiceBinding, error) {
	bindings := []v1alpha2.ServiceBinding{}
	if len(build.Services()) != 0 {
		for _, s := range build.Services() {
			if s.APIVersion == "v1" && s.Kind == "Secret" {
				bindings = append(bindings, v1alpha2.ServiceBinding{
					Name:      s.Name,
					SecretRef: &corev1.LocalObjectReference{Name: s.Name},
				})
				continue
			}
			gvr, _ := meta.UnsafeGuessKindToResource(s.GroupVersionKind())
			unstructured, err := g.DynamicClient.Resource(gvr).Namespace(build.GetNamespace()).Get(s.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			ps := duckprovisionedserviceable.ProvisionedServicable{}
			if err := duck.FromUnstructured(unstructured, &ps); err != nil {
				return nil, err
			}

			bindings = append(bindings, v1alpha2.ServiceBinding{
				Name:      s.Name,
				SecretRef: ps.Status.Binding,
			})
		}
	} else if v1alpha1Bindings, err := build.V1Alpha1Bindings(); err != nil {
		return nil, err
	} else if len(v1alpha1Bindings) != 0 {
		for _, b := range v1alpha1Bindings {
			bindings = append(bindings, v1alpha2.ServiceBinding{
				Name:                b.Name,
				SecretRef:           b.SecretRef,
				V1Alpha1MetadataRef: b.MetadataRef,
			})
		}
	}
	return bindings, nil
}

func parseCNBID(image ggcrv1.Image, env string) (int64, error) {
	v, err := imagehelpers.GetEnv(image, env)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}
