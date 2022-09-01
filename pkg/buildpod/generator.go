package buildpod

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
	"github.com/buildpacks/lifecycle/platform"
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

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/cnb"
	"github.com/pivotal/kpack/pkg/duckprovisionedserviceable"
	"github.com/pivotal/kpack/pkg/registry"
	"github.com/pivotal/kpack/pkg/registry/imagehelpers"
)

const (
	builderMetadataLabel = "io.buildpacks.builder.metadata"
	cnbUserId            = "CNB_USER_ID"
	cnbGroupId           = "CNB_GROUP_ID"
)

type ImageFetcher interface {
	Fetch(keychain authn.Keychain, repoName string) (ggcrv1.Image, string, error)
}

type Generator struct {
	BuildPodConfig            buildapi.BuildPodImages
	K8sClient                 k8sclient.Interface
	KeychainFactory           registry.KeychainFactory
	ImageFetcher              ImageFetcher
	DynamicClient             dynamic.Interface
	MaximumPlatformApiVersion *semver.Version
}

type BuildPodable interface {
	GetName() string
	GetNamespace() string
	ServiceAccount() string
	BuilderSpec() corev1alpha1.BuildBuilderSpec
	CnbBindings() corev1alpha1.CNBBindings
	Services() buildapi.Services

	BuildPod(buildapi.BuildPodImages, buildapi.BuildContext) (*corev1.Pod, error)
}

func (g *Generator) Generate(ctx context.Context, build BuildPodable) (*v1.Pod, error) {
	bindings, err := g.fetchServiceBindings(ctx, build)
	if err != nil {
		return nil, err
	}

	secrets, imagePullSecrets, err := g.fetchBuildSecrets(ctx, build)
	if err != nil {
		return nil, err
	}

	buildPodBuilderConfig, err := g.fetchBuilderConfig(ctx, build)
	if err != nil {
		return nil, err
	}

	return build.BuildPod(g.BuildPodConfig, buildapi.BuildContext{
		BuildPodBuilderConfig:     buildPodBuilderConfig,
		Secrets:                   secrets,
		Bindings:                  bindings,
		ImagePullSecrets:          imagePullSecrets,
		MaximumPlatformApiVersion: g.MaximumPlatformApiVersion,
	})
}

func (g *Generator) fetchServiceBindings(ctx context.Context, build BuildPodable) ([]buildapi.ServiceBinding, error) {
	serviceAccounts, err := g.fetchServiceAccounts(ctx, build)
	if err != nil {
		return nil, err
	}

	var forbiddenSecrets = map[string]struct{}{}
	for _, serviceAccount := range serviceAccounts {
		for _, secret := range serviceAccount.Secrets {
			forbiddenSecrets[secret.Name] = struct{}{}
		}
	}

	bindings := make([]buildapi.ServiceBinding, 0)

	if services := build.Services(); len(services) != 0 {
		for _, s := range services {
			var sb corev1alpha1.ServiceBinding
			if s.Kind == "Secret" {
				sb = corev1alpha1.ServiceBinding{
					Name:      s.Name,
					SecretRef: &corev1.LocalObjectReference{Name: s.Name},
				}
			} else {
				ps, err := g.readProvisionedServiceDuckType(ctx, build, s)
				if err != nil {
					return nil, err
				}
				sb = corev1alpha1.ServiceBinding{
					Name:      s.Name,
					SecretRef: ps.Status.Binding,
				}
			}

			if bindingUsesForbiddenSecret(forbiddenSecrets, sb.SecretRef) {
				return nil, errors.Errorf("build rejected: service %q uses forbidden secret %q", sb.Name, sb.SecretRef.Name)
			}

			bindings = append(bindings, &sb)
		}
		return bindings, nil
	}

	if cnbBindings := build.CnbBindings(); len(cnbBindings) != 0 {
		for _, b := range cnbBindings {
			if bindingUsesForbiddenSecret(forbiddenSecrets, b.SecretRef) {
				return nil, fmt.Errorf("build rejected: binding %q uses forbidden secret %q", b.Name, b.SecretRef.Name)
			}
			bindings = append(bindings, &corev1alpha1.CNBServiceBinding{
				Name:        b.Name,
				SecretRef:   b.SecretRef,
				MetadataRef: b.MetadataRef,
			})
		}
		return bindings, nil
	}

	return bindings, nil
}

func (g *Generator) readProvisionedServiceDuckType(ctx context.Context, build BuildPodable, s v1.ObjectReference) (duckprovisionedserviceable.ProvisionedServicable, error) {
	gvr, _ := meta.UnsafeGuessKindToResource(s.GroupVersionKind())
	unstructured, err := g.DynamicClient.Resource(gvr).Namespace(build.GetNamespace()).Get(ctx, s.Name, metav1.GetOptions{})
	if err != nil {
		return duckprovisionedserviceable.ProvisionedServicable{}, err
	}
	ps := duckprovisionedserviceable.ProvisionedServicable{}
	if err := duck.FromUnstructured(unstructured, &ps); err != nil {
		return duckprovisionedserviceable.ProvisionedServicable{}, err
	}
	return ps, nil
}

func bindingUsesForbiddenSecret(forbiddenSecrets map[string]struct{}, secretRef *corev1.LocalObjectReference) bool {
	if secretRef == nil {
		return false
	}

	_, ok := forbiddenSecrets[secretRef.Name]
	return ok
}

func (g *Generator) fetchServiceAccounts(ctx context.Context, build BuildPodable) ([]corev1.ServiceAccount, error) {
	serviceAccounts, err := g.K8sClient.CoreV1().ServiceAccounts(build.GetNamespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return []v1.ServiceAccount{}, err
	}
	return serviceAccounts.Items, nil
}

func (g *Generator) fetchBuildSecrets(ctx context.Context, build BuildPodable) ([]corev1.Secret, []corev1.LocalObjectReference, error) {
	var secrets []corev1.Secret
	var secretSet = map[string]struct{}{}
	serviceAccount, err := g.K8sClient.CoreV1().ServiceAccounts(build.GetNamespace()).Get(ctx, build.ServiceAccount(), metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	for _, secretRef := range serviceAccount.Secrets {
		if secretRef.Name == "" {
			return []corev1.Secret{}, []corev1.LocalObjectReference{}, errors.New("ServiceAccount has invalid Secret reference")
		}

		secret, err := g.K8sClient.CoreV1().Secrets(build.GetNamespace()).Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, nil, err
		}
		if _, ok := secretSet[secret.Name]; !ok {
			secrets = append(secrets, *secret)
			secretSet[secret.Name] = struct{}{}
		}
	}

	var imagePullSecrets []corev1.LocalObjectReference
	for _, secretRef := range serviceAccount.ImagePullSecrets {
		if secretRef.Name == "" {
			return []corev1.Secret{}, []corev1.LocalObjectReference{}, errors.New("ServiceAccount has invalid ImagePullSecret reference")
		}

		if _, ok := secretSet[secretRef.Name]; !ok {
			imagePullSecrets = append(imagePullSecrets, secretRef)
			secretSet[secretRef.Name] = struct{}{}
		}

	}

	return secrets, imagePullSecrets, nil
}

func (g *Generator) fetchBuilderConfig(ctx context.Context, build BuildPodable) (buildapi.BuildPodBuilderConfig, error) {
	keychain, err := g.KeychainFactory.KeychainForSecretRef(ctx, registry.SecretRef{
		Namespace:        build.GetNamespace(),
		ImagePullSecrets: build.BuilderSpec().ImagePullSecrets,
		ServiceAccount:   build.ServiceAccount(),
	})
	if err != nil {
		return buildapi.BuildPodBuilderConfig{}, errors.Wrap(err, "unable to create builder image keychain")
	}

	image, _, err := g.ImageFetcher.Fetch(keychain, build.BuilderSpec().Image)
	if err != nil {
		return buildapi.BuildPodBuilderConfig{}, errors.Wrap(err, "unable to fetch remote builder image")
	}

	stackId, err := imagehelpers.GetStringLabel(image, platform.StackIDLabel)
	if err != nil {
		return buildapi.BuildPodBuilderConfig{}, errors.Wrap(err, "builder image stack ID label not present")
	}

	var metadata cnb.BuilderImageMetadata
	err = imagehelpers.GetLabel(image, builderMetadataLabel, &metadata)
	if err != nil {
		return buildapi.BuildPodBuilderConfig{}, errors.Wrap(err, "unable to get builder metadata")
	}

	uid, err := parseCNBID(image, cnbUserId)
	if err != nil {
		return buildapi.BuildPodBuilderConfig{}, err
	}

	gid, err := parseCNBID(image, cnbGroupId)
	if err != nil {
		return buildapi.BuildPodBuilderConfig{}, err
	}

	config, err := image.ConfigFile()
	if err != nil {
		return buildapi.BuildPodBuilderConfig{}, err
	}

	return buildapi.BuildPodBuilderConfig{
		StackID:      stackId,
		RunImage:     metadata.Stack.RunImage.Image,
		PlatformAPIs: append(metadata.Lifecycle.APIs.Platform.Deprecated, metadata.Lifecycle.APIs.Platform.Supported...),
		Uid:          uid,
		Gid:          gid,
		OS:           config.OS,
	}, nil
}

func parseCNBID(image ggcrv1.Image, env string) (int64, error) {
	v, err := imagehelpers.GetEnv(image, env)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}
