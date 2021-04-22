package test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/logs"
	"github.com/pivotal/kpack/pkg/registry"
)

func TestCreateImage(t *testing.T) {
	rand.Seed(time.Now().Unix())

	spec.Run(t, "CreateImage", testCreateImage)
}

func testCreateImage(t *testing.T, when spec.G, it spec.S) {
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

	var (
		cfg     config
		clients *clients
		ctx     = context.Background()
	)

	it.Before(func() {
		cfg = loadConfig(t)

		var err error
		clients, err = newClients(t)
		require.NoError(t, err)

		err = clients.client.KpackV1alpha1().ClusterStores().Delete(ctx, clusterStoreName, metav1.DeleteOptions{})
		if !errors.IsNotFound(err) {
			require.NoError(t, err)
		}

		err = clients.client.KpackV1alpha1().ClusterStacks().Delete(ctx, clusterStackName, metav1.DeleteOptions{})
		if !errors.IsNotFound(err) {
			require.NoError(t, err)
		}

		err = clients.client.KpackV1alpha1().ClusterBuilders().Delete(ctx, clusterBuilderName, metav1.DeleteOptions{})
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
	})

	it.After(func() {
		for _, tag := range cfg.generatedImageNames {
			deleteImageTag(t, tag)
		}
	})

	it.Before(func() {
		reference, err := name.ParseReference(cfg.imageTag, name.WeakValidation)
		require.NoError(t, err)

		auth, err := authn.DefaultKeychain.Resolve(reference.Context().Registry)
		require.NoError(t, err)

		basicAuth, err := auth.Authorization()
		require.NoError(t, err)

		_, err = clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: dockerSecret,
				Annotations: map[string]string{
					"kpack.io/docker": reference.Context().RegistryStr(),
				},
			},
			StringData: map[string]string{
				"username": basicAuth.Username,
				"password": basicAuth.Password,
			},
			Type: corev1.SecretTypeBasicAuth,
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: serviceAccountName,
			},
			Secrets: []corev1.ObjectReference{
				{
					Name: dockerSecret,
				},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = clients.client.KpackV1alpha1().ClusterStores().Create(ctx, &v1alpha1.ClusterStore{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterStoreName,
			},
			Spec: v1alpha1.ClusterStoreSpec{
				Sources: []v1alpha1.StoreImage{
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

		_, err = clients.client.KpackV1alpha1().ClusterStacks().Create(ctx, &v1alpha1.ClusterStack{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterStackName,
			},
			Spec: v1alpha1.ClusterStackSpec{
				Id: "io.buildpacks.stacks.bionic",
				BuildImage: v1alpha1.ClusterStackSpecImage{
					Image: "gcr.io/paketo-buildpacks/build:base-cnb",
				},
				RunImage: v1alpha1.ClusterStackSpecImage{
					Image: "gcr.io/paketo-buildpacks/run:base-cnb",
				},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		builder, err := clients.client.KpackV1alpha1().Builders(testNamespace).Create(ctx, &v1alpha1.Builder{
			ObjectMeta: metav1.ObjectMeta{
				Name:      builderName,
				Namespace: testNamespace,
			},
			Spec: v1alpha1.NamespacedBuilderSpec{
				BuilderSpec: v1alpha1.BuilderSpec{
					Tag: cfg.newImageTag(),
					Stack: corev1.ObjectReference{
						Name: clusterStackName,
						Kind: "ClusterStack",
					},
					Store: corev1.ObjectReference{
						Name: clusterStoreName,
						Kind: "ClusterStore",
					},
					Order: []v1alpha1.OrderEntry{
						{
							Group: []v1alpha1.BuildpackRef{
								{
									BuildpackInfo: v1alpha1.BuildpackInfo{
										Id: "paketo-buildpacks/nodejs",
									},
								},
							},
						},
						{
							Group: []v1alpha1.BuildpackRef{
								{
									BuildpackInfo: v1alpha1.BuildpackInfo{
										Id: "paketo-buildpacks/bellsoft-liberica",
									},
								},
								{
									BuildpackInfo: v1alpha1.BuildpackInfo{
										Id: "paketo-buildpacks/gradle",
									},
									Optional: true,
								},
								{
									BuildpackInfo: v1alpha1.BuildpackInfo{
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

		clusterBuilder, err := clients.client.KpackV1alpha1().ClusterBuilders().Create(ctx, &v1alpha1.ClusterBuilder{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterBuilderName,
			},
			Spec: v1alpha1.ClusterBuilderSpec{
				BuilderSpec: v1alpha1.BuilderSpec{
					Tag: cfg.newImageTag(),
					Stack: corev1.ObjectReference{
						Name: clusterStackName,
						Kind: "ClusterStack",
					},
					Store: corev1.ObjectReference{
						Name: clusterStoreName,
						Kind: "ClusterStore",
					},
					Order: []v1alpha1.OrderEntry{
						{
							Group: []v1alpha1.BuildpackRef{
								{
									BuildpackInfo: v1alpha1.BuildpackInfo{
										Id: "paketo-buildpacks/nodejs",
									},
								},
							},
						},
						{
							Group: []v1alpha1.BuildpackRef{
								{
									BuildpackInfo: v1alpha1.BuildpackInfo{
										Id: "paketo-buildpacks/bellsoft-liberica",
									},
								},
								{
									BuildpackInfo: v1alpha1.BuildpackInfo{
										Id: "paketo-buildpacks/gradle",
									},
									Optional: true,
								},
								{
									BuildpackInfo: v1alpha1.BuildpackInfo{
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
	})

	it("builds and rebases git, blob, and registry based images", func() {

		cacheSize := resource.MustParse("1Gi")

		expectedResources := corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("1G"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("512M"),
			},
		}

		imageSources := map[string]v1alpha1.SourceConfig{
			"test-git-image": {
				Git: &v1alpha1.Git{
					URL:      "https://github.com/cloudfoundry-samples/cf-sample-app-nodejs",
					Revision: "master",
				},
			},
			"test-blob-image": {
				Blob: &v1alpha1.Blob{
					URL: "https://storage.googleapis.com/build-service/sample-apps/spring-petclinic-2.1.0.BUILD-SNAPSHOT.jar",
				},
			},
			"test-registry-image": {
				Registry: &v1alpha1.Registry{
					Image: "gcr.io/cf-build-service-public/fixtures/nodejs-source@sha256:76cb2e087b6f1355caa8ed4a5eebb1ad7376e26995a8d49a570cdc10e4976e44",
				},
			},
		}

		builderConfigs := map[string]corev1.ObjectReference{
			"custom-builder": {
				Kind: v1alpha1.BuilderKind,
				Name: builderName,
			},
			"custom-cluster-builder": {
				Kind: v1alpha1.ClusterBuilderKind,
				Name: clusterBuilderName,
			},
		}

		for imageType := range imageSources {
			for builderType := range builderConfigs {

				imageName := fmt.Sprintf("%s-%s", imageType, builderType)
				source := imageSources[imageType]
				builder := builderConfigs[builderType]

				t.Run(imageName, func(t *testing.T) {
					t.Parallel()

					imageTag := cfg.newImageTag()
					image, err := clients.client.KpackV1alpha1().Images(testNamespace).Create(ctx, &v1alpha1.Image{
						ObjectMeta: metav1.ObjectMeta{
							Name: imageName,
						},
						Spec: v1alpha1.ImageSpec{
							Tag:                  imageTag,
							Builder:              builder,
							ServiceAccount:       serviceAccountName,
							Source:               source,
							CacheSize:            &cacheSize,
							ImageTaggingStrategy: v1alpha1.None,
							Build: &v1alpha1.ImageBuild{
								Resources: expectedResources,
							},
						},
					}, metav1.CreateOptions{})
					require.NoError(t, err)

					validateImageCreate(t, clients, image, expectedResources)
					validateRebase(t, ctx, clients, image.Name, testNamespace)
				})
			}
		}
	})

	it("can trigger rebuilds", func() {
		cacheSize := resource.MustParse("1Gi")

		expectedResources := corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("1G"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("512M"),
			},
		}

		imageName := fmt.Sprintf("%s-%s", "test-git-image", "cluster-builder")

		imageTag := cfg.newImageTag()
		image, err := clients.client.KpackV1alpha1().Images(testNamespace).Create(ctx, &v1alpha1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: imageName,
			},
			Spec: v1alpha1.ImageSpec{
				Tag: imageTag,
				Builder: corev1.ObjectReference{
					Kind: v1alpha1.ClusterBuilderKind,
					Name: clusterBuilderName,
				},
				ServiceAccount: serviceAccountName,
				Source: v1alpha1.SourceConfig{
					Git: &v1alpha1.Git{
						URL:      "https://github.com/cloudfoundry-samples/cf-sample-app-nodejs",
						Revision: "master",
					},
				},
				CacheSize:            &cacheSize,
				ImageTaggingStrategy: v1alpha1.None,
				Build: &v1alpha1.ImageBuild{
					Resources: expectedResources,
				},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		validateImageCreate(t, clients, image, expectedResources)

		list, err := clients.client.KpackV1alpha1().Builds(testNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", imageName),
		})
		require.NoError(t, err)
		require.Len(t, list.Items, 1)

		build := &list.Items[0]
		build.Annotations[v1alpha1.BuildNeededAnnotation] = "2006-01-02 15:04:05.000000 -0700 MST m=+0.000000000"
		_, err = clients.client.KpackV1alpha1().Builds(testNamespace).Update(ctx, build, metav1.UpdateOptions{})
		require.NoError(t, err)

		eventually(t, func() bool {
			list, err := clients.client.KpackV1alpha1().Builds(testNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", imageName),
			})
			require.NoError(t, err)
			return len(list.Items) == 2
		}, 5*time.Second, 1*time.Minute)
	})
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

		eventually(t, func() bool {

			unstructured, err := clients.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			require.NoError(t, err)

			kResource := &duckv1.KResource{}
			err = duck.FromUnstructured(unstructured, kResource)
			require.NoError(t, err)

			return kResource.Status.GetCondition(apis.ConditionReady).IsTrue()
		}, 1*time.Second, 8*time.Minute)
	}
}

func validateImageCreate(t *testing.T, clients *clients, image *v1alpha1.Image, expectedResources corev1.ResourceRequirements) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logTail := &bytes.Buffer{}
	go func() {
		err := logs.NewBuildLogsClient(clients.k8sClient).Tail(ctx, logTail, image.Name, "1", image.Namespace)
		require.NoError(t, err)
	}()

	t.Logf("Waiting for image '%s' to be created", image.Name)
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

	buildList, err := clients.client.KpackV1alpha1().Builds(testNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", imageName),
	})
	require.NoError(t, err)

	require.Len(t, buildList.Items, 1)
	build := buildList.Items[0]

	rebaseBuildBuildSpec := build.Spec.DeepCopy()
	rebaseBuildBuildSpec.LastBuild = &v1alpha1.LastBuild{
		Image:   build.Status.LatestImage,
		StackId: build.Status.Stack.ID,
	}

	_, err = clients.client.KpackV1alpha1().Builds(testNamespace).Create(ctx, &v1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rebaseBuildName,
			Annotations: map[string]string{v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonStack},
		},
		Spec: *rebaseBuildBuildSpec,
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	eventually(t, func() bool {
		build, err := clients.client.KpackV1alpha1().Builds(testNamespace).Get(ctx, rebaseBuildName, metav1.GetOptions{})
		require.NoError(t, err)

		require.LessOrEqual(t, len(build.Status.StepsCompleted), 1)

		return build.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsTrue()
	}, 5*time.Second, 1*time.Minute)
}

func deleteImageTag(t *testing.T, deleteImageTag string) {
	reference, err := name.ParseReference(deleteImageTag, name.WeakValidation)
	require.NoError(t, err)

	authenticator, err := authn.DefaultKeychain.Resolve(reference.Context().Registry)
	require.NoError(t, err)

	err = remote.Delete(reference, remote.WithAuth(authenticator))
	require.NoError(t, err)
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
