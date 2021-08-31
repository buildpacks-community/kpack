package test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned"
	"github.com/pivotal/kpack/pkg/logs"
	"github.com/pivotal/kpack/pkg/registry"
)

var (
	setup         sync.Once
	client        *versioned.Clientset
	k8sClient     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	clusterConfig *rest.Config
	err           error
)

const (
	testNamespace      = "test"
	dockerSecret       = "docker-secret"
	serviceAccountName = "image-service-account"
	builderImage       = "gcr.io/paketo-buildpacks/builder:base"
	clusterStoreName   = "store"
	clusterStackName   = "stack"
	builderName        = "custom-builder"
	clusterBuilderName = "custom-cluster-builder"
)

func newClients(t *testing.T) (*clients, error) {
	setup.Do(func() {
		kubeconfig := flag.String("kubeconfig", getKubeConfig(), "Path to a kubeconfig. Only required if out-of-cluster.")
		masterURL := flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")

		flag.Parse()

		clusterConfig, err = clientcmd.BuildConfigFromFlags(*masterURL, *kubeconfig)
		require.NoError(t, err)

		client, err = versioned.NewForConfig(clusterConfig)
		require.NoError(t, err)

		k8sClient, err = kubernetes.NewForConfig(clusterConfig)
		require.NoError(t, err)

		dynamicClient, err = dynamic.NewForConfig(clusterConfig)
		require.NoError(t, err)
	})

	return &clients{
		client:        client,
		k8sClient:     k8sClient,
		dynamicClient: dynamicClient,
	}, nil
}

func getKubeConfig() string {
	if config, found := os.LookupEnv("KUBECONFIG"); found {
		return config
	}
	if usr, err := user.Current(); err == nil {
		return path.Join(usr.HomeDir, ".kube/config")
	}
	return ""
}

type clients struct {
	client        versioned.Interface
	k8sClient     kubernetes.Interface
	dynamicClient dynamic.Interface
}

func prepareTestWorkspace(t *testing.T, ctx context.Context, clients *clients, cfg config) {
	t.Log("Cleaning up old resources...")
	err = clients.client.KpackV1alpha2().ClusterStores().Delete(ctx, clusterStoreName, metav1.DeleteOptions{})
	if !errors.IsNotFound(err) {
		require.NoError(t, err)
	}

	err = clients.client.KpackV1alpha2().ClusterStacks().Delete(ctx, clusterStackName, metav1.DeleteOptions{})
	if !errors.IsNotFound(err) {
		require.NoError(t, err)
	}

	err = clients.client.KpackV1alpha2().ClusterBuilders().Delete(ctx, clusterBuilderName, metav1.DeleteOptions{})
	if !errors.IsNotFound(err) {
		require.NoError(t, err)
	}

	deleteNamespace(t, ctx, clients, testNamespace)

	_, err = clients.k8sClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   testNamespace,
			Labels: readNamespaceLabelsFromEnv(),
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	reference, err := name.ParseReference(cfg.testRegistry, name.WeakValidation)
	require.NoError(t, err)

	registry := reference.Context().Registry
	authenticator, err := authn.DefaultKeychain.Resolve(registry)
	require.NoError(t, err)

	auth, err := authenticator.Authorization()
	require.NoError(t, err)

	secrets := []corev1.ObjectReference{}
	secret := createSecret(dockerSecret, registry.RegistryStr(), auth)
	if secret != nil {
		t.Log("Creating registry secret...")
		_, err = clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, secret, metav1.CreateOptions{})
		require.NoError(t, err)
		secrets = append(secrets, corev1.ObjectReference{
			Name: dockerSecret,
		})
	}

	t.Log("Creating service account...")
	_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceAccountName,
		},
		Secrets: secrets,
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Log("Creating cluster store...")
	clusterStore, err := clients.client.KpackV1alpha2().ClusterStores().Create(ctx, &buildapi.ClusterStore{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterStoreName,
		},
		Spec: buildapi.ClusterStoreSpec{
			Sources: []corev1alpha1.StoreImage{
				{
					Image: builderImage,
				},
				{
					Image: "gcr.io/paketo-buildpacks/gradle",
				},
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	waitUntilReady(t, ctx, clients, clusterStore)

	t.Log("Creating cluster stack...")
	_, err = clients.client.KpackV1alpha2().ClusterStacks().Create(ctx, &buildapi.ClusterStack{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterStackName,
		},
		Spec: buildapi.ClusterStackSpec{
			Id: "io.buildpacks.stacks.bionic",
			BuildImage: buildapi.ClusterStackSpecImage{
				Image: "gcr.io/paketo-buildpacks/build:base-cnb",
			},
			RunImage: buildapi.ClusterStackSpecImage{
				Image: "gcr.io/paketo-buildpacks/run:base-cnb",
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Log("Creating builder...")
	builder, err := clients.client.KpackV1alpha2().Builders(testNamespace).Create(ctx, &buildapi.Builder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builderName,
			Namespace: testNamespace,
		},
		Spec: buildapi.NamespacedBuilderSpec{
			BuilderSpec: buildapi.BuilderSpec{
				Tag: cfg.newImageTag(),
				Stack: corev1.ObjectReference{
					Name: clusterStackName,
					Kind: "ClusterStack",
				},
				Store: corev1.ObjectReference{
					Name: clusterStoreName,
					Kind: "ClusterStore",
				},
				Order: []corev1alpha1.OrderEntry{
					{
						Group: []corev1alpha1.BuildpackRef{
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id: "paketo-buildpacks/nodejs",
								},
							},
						},
					},
					{
						Group: []corev1alpha1.BuildpackRef{
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id: "paketo-buildpacks/bellsoft-liberica",
								},
							},
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id: "paketo-buildpacks/gradle",
								},
								Optional: true,
							},
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id: "paketo-buildpacks/executable-jar",
								},
							},
						},
					},
				},
			},
			ServiceAccount: serviceAccountName,
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	t.Log("Creating cluster builder...")
	clusterBuilder, err := clients.client.KpackV1alpha2().ClusterBuilders().Create(ctx, &buildapi.ClusterBuilder{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterBuilderName,
		},
		Spec: buildapi.ClusterBuilderSpec{
			BuilderSpec: buildapi.BuilderSpec{
				Tag: cfg.newImageTag(),
				Stack: corev1.ObjectReference{
					Name: clusterStackName,
					Kind: "ClusterStack",
				},
				Store: corev1.ObjectReference{
					Name: clusterStoreName,
					Kind: "ClusterStore",
				},
				Order: []corev1alpha1.OrderEntry{
					{
						Group: []corev1alpha1.BuildpackRef{
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id: "paketo-buildpacks/nodejs",
								},
							},
						},
					},
					{
						Group: []corev1alpha1.BuildpackRef{
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id: "paketo-buildpacks/bellsoft-liberica",
								},
							},
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id: "paketo-buildpacks/gradle",
								},
								Optional: true,
							},
							{
								BuildpackInfo: corev1alpha1.BuildpackInfo{
									Id: "paketo-buildpacks/executable-jar",
								},
							},
						},
					},
				},
			},
			ServiceAccountRef: corev1.ObjectReference{
				Namespace: testNamespace,
				Name:      serviceAccountName,
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	waitUntilReady(t, ctx, clients, builder, clusterBuilder)
}

func deleteCreatedImages(t *testing.T, cfg config) {
	for _, tag := range cfg.generatedImageNames {
		t.Logf("Deleting image: %s", tag)
		err := deleteImageTag(tag)
		if err != nil {
			t.Error(err)
		}
	}

	// remove state
	cfg.generatedImageNames = []string{}
}

func readNamespaceLabelsFromEnv() map[string]string {
	labelsToSet := map[string]string{}
	if labelsStrToSet, found := os.LookupEnv("KPACK_TEST_NAMESPACE_LABELS"); found {
		labels := strings.Split(labelsStrToSet, ",")
		for _, str := range labels {
			parts := strings.Split(str, "=")
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]
				labelsToSet[key] = value
			}

		}
	}
	return labelsToSet
}

func waitUntilReady(t *testing.T, ctx context.Context, clients *clients, objects ...kmeta.OwnerRefable) {
	for _, ob := range objects {
		namespace := ob.GetObjectMeta().GetNamespace()
		name := ob.GetObjectMeta().GetName()
		gvr, _ := meta.UnsafeGuessKindToResource(ob.GetGroupVersionKind())

		t.Logf("Waiting for [ns=%s,name=%s,kind=%s,version=%s] to be ready...", namespace, name, gvr.Resource, gvr.Version)
		eventually(t, func() bool {
			unstructured, err := clients.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			require.NoError(t, err)

			kResource := &duckv1.KResource{}
			err = duck.FromUnstructured(unstructured, kResource)
			require.NoError(t, err)

			return kResource.Status.GetCondition(apis.ConditionReady).IsTrue()
		}, 1*time.Second, 15*time.Minute)
	}
}

func validateImageCreate(t *testing.T, clients *clients, image *buildapi.Image, expectedResources corev1.ResourceRequirements) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logTail := &bytes.Buffer{}
	go func() {
		err := logs.NewBuildLogsClient(clients.k8sClient).Tail(ctx, logTail, image.Name, "1", image.Namespace)
		require.NoError(t, err)
	}()

	waitUntilReady(t, ctx, clients, image)

	registryClient := &registry.Client{}
	_, _, err = registryClient.Fetch(authn.DefaultKeychain, image.Spec.Tag)
	require.NoError(t, err)

	eventually(t, func() bool {
		return strings.Contains(logTail.String(), "Build successful")
	}, 1*time.Second, 10*time.Second)

	podList, err := clients.k8sClient.CoreV1().Pods(image.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", image.Name),
	})
	require.NoError(t, err)

	require.Len(t, podList.Items, 1)
	pod := podList.Items[0]

	require.Equal(t, 1, len(pod.Spec.Containers))
	assert.Equal(t, expectedResources, pod.Spec.Containers[0].Resources)
}

func validateRebase(t *testing.T, ctx context.Context, clients *clients, imageName, testNamespace string) {
	var rebaseBuildName = imageName + "-rebase"

	buildList, err := clients.client.KpackV1alpha2().Builds(testNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", imageName),
	})
	require.NoError(t, err)

	require.Len(t, buildList.Items, 1)
	build := buildList.Items[0]

	rebaseBuildBuildSpec := build.Spec.DeepCopy()
	rebaseBuildBuildSpec.LastBuild = &buildapi.LastBuild{
		Image:   build.Status.LatestImage,
		StackId: build.Status.Stack.ID,
	}

	_, err = clients.client.KpackV1alpha2().Builds(testNamespace).Create(ctx, &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rebaseBuildName,
			Annotations: map[string]string{buildapi.BuildReasonAnnotation: buildapi.BuildReasonStack},
		},
		Spec: *rebaseBuildBuildSpec,
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	eventually(t, func() bool {
		build, err := clients.client.KpackV1alpha2().Builds(testNamespace).Get(ctx, rebaseBuildName, metav1.GetOptions{})
		require.NoError(t, err)

		require.LessOrEqual(t, len(build.Status.StepsCompleted), 1)

		return build.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsTrue()
	}, 5*time.Second, 2*time.Minute)
}

func deleteImageTag(image string) error {
	reference, err := name.ParseReference(image, name.WeakValidation)
	if err != nil {
		return err
	}

	authenticator, err := authn.DefaultKeychain.Resolve(reference.Context().Registry)
	if err != nil {
		return err
	}

	switch reference.(type) {
	case name.Tag:
		// need to get digest for deletion (see https://github.com/google/go-containerregistry/issues/999#issuecomment-828701797)
		head, err := remote.Head(reference, remote.WithAuth(authenticator))
		if err != nil {
			return err
		}

		reference, err = name.ParseReference(fmt.Sprintf("%s@%s", reference.Name(), head.Digest.String()), name.WeakValidation)
		if err != nil {
			return err
		}
	}

	err = remote.Delete(reference, remote.WithAuth(authenticator))
	if err != nil {
		return err
	}

	return nil
}

func deleteNamespace(t *testing.T, ctx context.Context, clients *clients, namespace string) {
	err := clients.k8sClient.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
	require.True(t, err == nil || errors.IsNotFound(err))
	if errors.IsNotFound(err) {
		return
	}

	var (
		timeout int64 = 120
		closed        = false
	)

	watcher, err := clients.k8sClient.CoreV1().Namespaces().Watch(ctx, metav1.ListOptions{
		TimeoutSeconds: &timeout,
	})
	require.NoError(t, err)

	for evt := range watcher.ResultChan() {
		if evt.Type != watch.Deleted {
			continue
		}
		if ns, ok := evt.Object.(*corev1.Namespace); ok {
			if ns.Name == namespace {
				closed = true
				break
			}
		}
	}
	require.True(t, closed)
}

func createSecret(name, registry string, auth *authn.AuthConfig) *corev1.Secret {
	if auth == nil {
		return nil
	}

	if auth.Username != "" && auth.Password != "" {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Annotations: map[string]string{
					"kpack.io/docker": registry,
				},
			},
			StringData: map[string]string{
				"username": auth.Username,
				"password": auth.Password,
			},
			Type: corev1.SecretTypeBasicAuth,
		}
	}

	return nil
}
