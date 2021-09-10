package buildpod

import (
	"context"
	"fmt"
	"sort"
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

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
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
	BuildPodConfig  buildapi.BuildPodImages
	K8sClient       k8sclient.Interface
	KeychainFactory registry.KeychainFactory
	ImageFetcher    ImageFetcher
	DynamicClient   dynamic.Interface
}

type BuildPodable interface {
	GetName() string
	GetNamespace() string
	ServiceAccount() string
	BuilderSpec() corev1alpha1.BuildBuilderSpec
	CnbBindings() corev1alpha1.CnbBindings
	Services() buildapi.Services

	BuildPod(buildapi.BuildPodImages, []corev1.Secret, []corev1.Taint, buildapi.BuildPodBuilderConfig, []buildapi.ServiceBinding) (*corev1.Pod, error)
}

func (g *Generator) Generate(ctx context.Context, build BuildPodable) (*v1.Pod, error) {
	bindings, err := g.fetchServiceBindings(ctx, build)
	if err != nil {
		return nil, err
	}

	secrets, err := g.fetchBuildSecrets(ctx, build)
	if err != nil {
		return nil, err
	}

	buildPodBuilderConfig, err := g.fetchBuilderConfig(ctx, build)
	if err != nil {
		return nil, err
	}

	taints, err := g.calculateHomogenousWindowsNodeTaints(ctx, buildPodBuilderConfig.OS)
	if err != nil {
		return nil, err
	}

	return build.BuildPod(g.BuildPodConfig, secrets, taints, buildPodBuilderConfig, bindings)
}

func (g *Generator) fetchServiceBindings(ctx context.Context, build BuildPodable) ([]buildapi.ServiceBinding, error) {
	serviceAccounts, err := g.fetchServiceAccounts(ctx, build)
	if err != nil {
		return nil, err
	}

	var forbiddenSecrets = map[string]bool{}
	for _, serviceAccount := range serviceAccounts {
		for _, secret := range serviceAccount.Secrets {
			forbiddenSecrets[secret.Name] = true
		}
	}

	bindings := make([]buildapi.ServiceBinding, 0)

	if services := build.Services(); len(services) != 0 {
		for _, s := range build.Services() {
			var b corev1alpha1.ServiceBinding
			if s.Kind == "Secret" {
				b = corev1alpha1.ServiceBinding{
					Name:      s.Name,
					SecretRef: &corev1.LocalObjectReference{Name: s.Name},
				}
			} else {
				gvr, _ := meta.UnsafeGuessKindToResource(s.GroupVersionKind())
				unstructured, err := g.DynamicClient.Resource(gvr).Namespace(build.GetNamespace()).Get(ctx, s.Name, metav1.GetOptions{})
				if err != nil {
					return nil, err
				}
				ps := duckprovisionedserviceable.ProvisionedServicable{}
				if err := duck.FromUnstructured(unstructured, &ps); err != nil {
					return nil, err
				}
				b = corev1alpha1.ServiceBinding{
					Name:      s.Name,
					SecretRef: ps.Status.Binding,
				}
			}
			if forbiddenSecrets[b.SecretRef.Name] {
				return nil, fmt.Errorf("build rejected: service %q uses forbidden secret %q", b.Name, b.SecretRef.Name)
			}

			secret, err := g.K8sClient.CoreV1().Secrets(build.GetNamespace()).Get(ctx, b.SecretRef.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			if t, ok := secret.StringData["type"]; !ok || secret.Type == "" || fmt.Sprintf("service.binding/%s", t) != string(secret.Type) {
				return nil, fmt.Errorf("build rejected: service secret %q does not contain required type (%q) and matching stringData.type (%q)", b.Name, secret.Type, t)
			}

			bindings = append(bindings, &b)
		}
		return bindings, nil
	}

	if cnbBindings := build.CnbBindings(); len(cnbBindings) != 0 {
		for _, s := range cnbBindings {
			if forbiddenSecrets[s.SecretRef.Name] {
				return nil, fmt.Errorf("build rejected: binding %q uses forbidden secret %q", s.Name, s.SecretRef.Name)
			}
			bindings = append(bindings, &corev1alpha1.CnbServiceBinding{
				Name:        s.Name,
				SecretRef:   s.SecretRef,
				MetadataRef: s.MetadataRef,
			})
		}
		return bindings, nil
	}

	return bindings, nil
}

func (g *Generator) fetchServiceAccounts(ctx context.Context, build BuildPodable) ([]corev1.ServiceAccount, error) {
	serviceAccounts, err := g.K8sClient.CoreV1().ServiceAccounts(build.GetNamespace()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return []v1.ServiceAccount{}, err
	}
	return serviceAccounts.Items, nil
}

func (g *Generator) fetchBuildSecrets(ctx context.Context, build BuildPodable) ([]corev1.Secret, error) {
	var secrets []corev1.Secret
	var secretSet = map[string]struct{}{}
	serviceAccount, err := g.K8sClient.CoreV1().ServiceAccounts(build.GetNamespace()).Get(ctx, build.ServiceAccount(), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, secretRef := range serviceAccount.Secrets {
		secret, err := g.K8sClient.CoreV1().Secrets(build.GetNamespace()).Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if _, ok := secretSet[secret.Name]; !ok {
			secrets = append(secrets, *secret)
			secretSet[secret.Name] = struct{}{}
		}
	}
	return secrets, nil
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

	stackId, err := imagehelpers.GetStringLabel(image, lifecycle.StackIDLabel)
	if err != nil {
		return buildapi.BuildPodBuilderConfig{}, errors.Wrap(err, "builder image stack ID label not present")
	}

	var metadata cnb.BuilderImageMetadata
	err = imagehelpers.GetLabel(image, cnb.BuilderMetadataLabel, &metadata)
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

func (g *Generator) calculateHomogenousWindowsNodeTaints(ctx context.Context, os string) ([]v1.Taint, error) {
	if os != "windows" {
		return nil, nil
	}

	windowsNodes, err := g.K8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "kubernetes.io/os=windows"})
	if err != nil {
		return nil, err
	}

	nodes := windowsNodes.Items
	if len(nodes) == 0 {
		return []v1.Taint{}, nil
	}

	taints := nodes[0].Spec.Taints
	sort.Slice(taints, func(i, j int) bool {
		return taints[i].Key < taints[j].Key
	})

	for _, node := range nodes[1:] {
		taintsToCompare := node.Spec.Taints
		sort.Slice(taintsToCompare, func(i, j int) bool {
			return taintsToCompare[i].Key < taintsToCompare[j].Key
		})

		if !taintsEqual(taints, taintsToCompare) {
			return []v1.Taint{}, nil
		}
	}

	return taints, nil
}

func taintsEqual(taint1, taint2 []v1.Taint) bool {
	if len(taint1) != len(taint2) {
		return false
	}

	for i := range taint2 {
		if (taint1[i].Key != taint2[i].Key) ||
			(taint1[i].Value != taint2[i].Value) ||
			(taint1[i].Effect != taint2[i].Effect) {
			return false
		}
	}

	return true
}
