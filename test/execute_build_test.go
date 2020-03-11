package test

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"knative.dev/pkg/apis/duck"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/kmeta"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	expv1alpha1 "github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1"
	"github.com/pivotal/kpack/pkg/logs"
	"github.com/pivotal/kpack/pkg/registry"
)

func TestCreateImage(t *testing.T) {
	rand.Seed(time.Now().Unix())

	spec.Run(t, "CreateImage", testCreateImage)
}

func testCreateImage(t *testing.T, when spec.G, it spec.S) {
	const (
		testNamespace            = "test"
		dockerSecret             = "docker-secret"
		builderName              = "build-service-builder"
		clusterBuilderName       = "cluster-builder"
		serviceAccountName       = "image-service-account"
		builderImage             = "cloudfoundry/cnb:bionic"
		storeName                = "store"
		stackName                = "stack"
		customBuilderName        = "custom-builder"
		customClusterBuilderName = "custom-cluster-builder"
	)

	var (
		cfg     config
		clients *clients
	)

	it.Before(func() {
		cfg = loadConfig(t)

		var err error
		clients, err = newClients(t)
		require.NoError(t, err)

		err = clients.client.ExperimentalV1alpha1().Stores().Delete(storeName, &metav1.DeleteOptions{})
		if !errors.IsNotFound(err) {
			require.NoError(t, err)
		}

		err = clients.client.ExperimentalV1alpha1().Stacks().Delete(stackName, &metav1.DeleteOptions{})
		if !errors.IsNotFound(err) {
			require.NoError(t, err)
		}

		err = clients.client.BuildV1alpha1().ClusterBuilders().Delete(clusterBuilderName, &metav1.DeleteOptions{})
		if !errors.IsNotFound(err) {
			require.NoError(t, err)
		}

		err = clients.client.ExperimentalV1alpha1().CustomClusterBuilders().Delete(customClusterBuilderName, &metav1.DeleteOptions{})
		if !errors.IsNotFound(err) {
			require.NoError(t, err)
		}

		deleteNamespace(t, clients, testNamespace)

		_, err = clients.k8sClient.CoreV1().Namespaces().Create(&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		})
		require.NoError(t, err)
	})

	it.After(func() {
		for _, tag := range cfg.generatedImageNames {
			deleteImageTag(t, tag)
		}
	})

	it("builds and rebases git, blob, and registry based images", func() {
		reference, err := name.ParseReference(cfg.imageTag, name.WeakValidation)
		require.NoError(t, err)

		auth, err := authn.DefaultKeychain.Resolve(reference.Context().Registry)
		require.NoError(t, err)

		basicAuth, err := auth.Authorization()
		require.NoError(t, err)

		_, err = clients.k8sClient.CoreV1().Secrets(testNamespace).Create(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: dockerSecret,
				Annotations: map[string]string{
					"build.pivotal.io/docker": reference.Context().RegistryStr(),
				},
			},
			StringData: map[string]string{
				"username": basicAuth.Username,
				"password": basicAuth.Password,
			},
			Type: v1.SecretTypeBasicAuth,
		})
		require.NoError(t, err)

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Create(&v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: serviceAccountName,
			},
			Secrets: []v1.ObjectReference{
				{
					Name: dockerSecret,
				},
			},
		})
		require.NoError(t, err)

		_, err = clients.client.ExperimentalV1alpha1().Stores().Create(&expv1alpha1.Store{
			ObjectMeta: metav1.ObjectMeta{
				Name: storeName,
			},
			Spec: expv1alpha1.StoreSpec{
				Sources: []expv1alpha1.StoreImage{
					{
						Image: builderImage,
					},
				},
			},
		})
		require.NoError(t, err)

		_, err = clients.client.ExperimentalV1alpha1().Stacks().Create(&expv1alpha1.Stack{
			ObjectMeta: metav1.ObjectMeta{
				Name: stackName,
			},
			Spec: expv1alpha1.StackSpec{
				Id: "io.buildpacks.stacks.bionic",
				BuildImage: expv1alpha1.StackSpecImage{
					Image: "cloudfoundry/build:base-cnb",
				},
				RunImage: expv1alpha1.StackSpecImage{
					Image: "cloudfoundry/run:base-cnb",
				},
			},
		})
		require.NoError(t, err)

		clusterBuilder, err := clients.client.BuildV1alpha1().ClusterBuilders().Create(&v1alpha1.ClusterBuilder{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterBuilderName,
			},
			Spec: v1alpha1.BuilderSpec{
				Image: builderImage,
			},
		})
		require.NoError(t, err)

		customBuilder, err := clients.client.ExperimentalV1alpha1().CustomBuilders(testNamespace).Create(&expv1alpha1.CustomBuilder{
			ObjectMeta: metav1.ObjectMeta{
				Name:      customBuilderName,
				Namespace: testNamespace,
			},
			Spec: expv1alpha1.CustomNamespacedBuilderSpec{
				CustomBuilderSpec: expv1alpha1.CustomBuilderSpec{
					Tag:   cfg.newImageTag(),
					Stack: stackName,
					Store: storeName,
					Order: []expv1alpha1.OrderEntry{
						{
							Group: []expv1alpha1.BuildpackRef{
								{
									BuildpackInfo: expv1alpha1.BuildpackInfo{
										Id: "org.cloudfoundry.nodejs",
									},
								},
							},
						},
						{
							Group: []expv1alpha1.BuildpackRef{
								{
									BuildpackInfo: expv1alpha1.BuildpackInfo{
										Id: "org.cloudfoundry.openjdk",
									},
								},
								{
									BuildpackInfo: expv1alpha1.BuildpackInfo{
										Id: "org.cloudfoundry.buildsystem",
									},
									Optional: true,
								},
								{
									BuildpackInfo: expv1alpha1.BuildpackInfo{
										Id: "org.cloudfoundry.jvmapplication",
									},
								},
							},
						},
					},
				},
				ServiceAccount: serviceAccountName,
			},
		})
		require.NoError(t, err)

		customClusterBuilder, err := clients.client.ExperimentalV1alpha1().CustomClusterBuilders().Create(&expv1alpha1.CustomClusterBuilder{
			ObjectMeta: metav1.ObjectMeta{
				Name: customClusterBuilderName,
			},
			Spec: expv1alpha1.CustomClusterBuilderSpec{
				CustomBuilderSpec: expv1alpha1.CustomBuilderSpec{
					Tag:   cfg.newImageTag(),
					Stack: stackName,
					Store: storeName,
					Order: []expv1alpha1.OrderEntry{
						{
							Group: []expv1alpha1.BuildpackRef{
								{
									BuildpackInfo: expv1alpha1.BuildpackInfo{
										Id: "org.cloudfoundry.nodejs",
									},
								},
							},
						},
						{
							Group: []expv1alpha1.BuildpackRef{
								{
									BuildpackInfo: expv1alpha1.BuildpackInfo{
										Id: "org.cloudfoundry.openjdk",
									},
								},
								{
									BuildpackInfo: expv1alpha1.BuildpackInfo{
										Id: "org.cloudfoundry.buildsystem",
									},
									Optional: true,
								},
								{
									BuildpackInfo: expv1alpha1.BuildpackInfo{
										Id: "org.cloudfoundry.jvmapplication",
									},
								},
							},
						},
					},
				},
				ServiceAccountRef: v1.ObjectReference{
					Namespace: testNamespace,
					Name:      serviceAccountName,
				},
			},
		})
		require.NoError(t, err)

		builder, err := clients.client.BuildV1alpha1().Builders(testNamespace).Create(&v1alpha1.Builder{
			ObjectMeta: metav1.ObjectMeta{
				Name:      builderName,
				Namespace: testNamespace,
			},
			Spec: v1alpha1.BuilderWithSecretsSpec{
				BuilderSpec:      v1alpha1.BuilderSpec{Image: builderImage},
				ImagePullSecrets: nil,
			},
		})
		require.NoError(t, err)

		waitUntilReady(t, clients, builder, customBuilder, clusterBuilder, customClusterBuilder)

		cacheSize := resource.MustParse("1Gi")

		expectedResources := v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("1G"),
			},
			Requests: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("512M"),
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
					Image: "gcr.io/cf-build-service-public/test-fixtures/registry-source@sha256:1140e5a0761c2b07ff98b605407b2bc03ac78831a59da4339ed976ed65700364",
				},
			},
		}

		builderConfigs := map[string]corev1.ObjectReference{
			"custom-builder": {
				Kind:       expv1alpha1.CustomBuilderKind,
				APIVersion: "experimental.kpack.pivotal.io/v1alpha1",
				Name:       customBuilderName,
			},
			"builder": {
				Kind:       v1alpha1.BuilderKind,
				APIVersion: "build.pivotal.io/v1alpha1",
				Name:       builderName,
			},
			"cluster-builder": {
				Kind:       v1alpha1.ClusterBuilderKind,
				APIVersion: "build.pivotal.io/v1alpha1",
				Name:       clusterBuilderName,
			},
			"custom-cluster-builder": {
				Kind:       expv1alpha1.CustomClusterBuilderKind,
				APIVersion: "build.pivotal.io/v1alpha1",
				Name:       customClusterBuilderName,
			},
		}

		for imageType := range imageSources {
			for builderType := range builderConfigs {

				imageName := fmt.Sprintf("%s-%s", imageType, builderType)
				source := imageSources[imageType]
				builder := builderConfigs[builderType]

				var wg sync.WaitGroup
				t.Run(imageName, func(t *testing.T) {
					wg.Add(1)
					t.Parallel()

					imageTag := cfg.newImageTag()
					image, err := clients.client.BuildV1alpha1().Images(testNamespace).Create(&v1alpha1.Image{
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
					})
					require.NoError(t, err)

					validateImageCreate(t, clients, image, expectedResources)
					wg.Done()
					wg.Wait()
					validateRebase(t, clients, image.Name, testNamespace)
				})
			}
		}
	})

	it("can trigger rebuilds", func() {
		reference, err := name.ParseReference(cfg.imageTag, name.WeakValidation)
		require.NoError(t, err)

		auth, err := authn.DefaultKeychain.Resolve(reference.Context().Registry)
		require.NoError(t, err)

		basicAuth, err := auth.Authorization()
		require.NoError(t, err)

		_, err = clients.k8sClient.CoreV1().Secrets(testNamespace).Create(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: dockerSecret,
				Annotations: map[string]string{
					"build.pivotal.io/docker": reference.Context().RegistryStr(),
				},
			},
			StringData: map[string]string{
				"username": basicAuth.Username,
				"password": basicAuth.Password,
			},
			Type: v1.SecretTypeBasicAuth,
		})
		require.NoError(t, err)

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Create(&v1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: serviceAccountName,
			},
			Secrets: []v1.ObjectReference{
				{
					Name: dockerSecret,
				},
			},
		})
		require.NoError(t, err)

		clusterBuilder, err := clients.client.BuildV1alpha1().ClusterBuilders().Create(&v1alpha1.ClusterBuilder{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterBuilderName,
			},
			Spec: v1alpha1.BuilderSpec{
				Image: builderImage,
			},
		})
		require.NoError(t, err)

		waitUntilReady(t, clients, clusterBuilder)

		cacheSize := resource.MustParse("1Gi")

		expectedResources := v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("1G"),
			},
			Requests: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("512M"),
			},
		}

		imageName := fmt.Sprintf("%s-%s", "test-git-image", "cluster-builder")

		imageTag := cfg.newImageTag()
		image, err := clients.client.BuildV1alpha1().Images(testNamespace).Create(&v1alpha1.Image{
			ObjectMeta: metav1.ObjectMeta{
				Name: imageName,
			},
			Spec: v1alpha1.ImageSpec{
				Tag: imageTag,
				Builder: corev1.ObjectReference{
					Kind:       v1alpha1.ClusterBuilderKind,
					APIVersion: "build.pivotal.io/v1alpha1",
					Name:       clusterBuilderName,
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
		})
		require.NoError(t, err)

		validateImageCreate(t, clients, image, expectedResources)

		list, err := clients.client.BuildV1alpha1().Builds(testNamespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("image.build.pivotal.io/image=%s", imageName),
		})
		require.NoError(t, err)
		require.Len(t, list.Items, 1)

		build := &list.Items[0]
		build.Annotations[v1alpha1.BuildNeededAnnotation] = "true"
		_, err = clients.client.BuildV1alpha1().Builds(testNamespace).Update(build)
		require.NoError(t, err)

		eventually(t, func() bool {
			list, err := clients.client.BuildV1alpha1().Builds(testNamespace).List(metav1.ListOptions{
				LabelSelector: fmt.Sprintf("image.build.pivotal.io/image=%s", imageName),
			})
			require.NoError(t, err)
			return len(list.Items) == 2
		}, 5*time.Second, 1*time.Minute)
	})
}

func waitUntilReady(t *testing.T, clients *clients, objects ...kmeta.OwnerRefable) {
	for _, ob := range objects {
		namespace := ob.GetObjectMeta().GetNamespace()
		name := ob.GetObjectMeta().GetName()
		gvr, _ := meta.UnsafeGuessKindToResource(ob.GetGroupVersionKind())

		eventually(t, func() bool {
			unstructured, err := clients.dynamicClient.Resource(gvr).Namespace(namespace).Get(name, metav1.GetOptions{})
			require.NoError(t, err)

			kResource := &duckv1alpha1.KResource{}
			err = duck.FromUnstructured(unstructured, kResource)
			require.NoError(t, err)

			return kResource.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue()
		}, 1*time.Second, 9*time.Minute)
	}
}

func validateImageCreate(t *testing.T, clients *clients, image *v1alpha1.Image, expectedResources v1.ResourceRequirements) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logTail := &bytes.Buffer{}
	go func() {
		err := logs.NewBuildLogsClient(clients.k8sClient).Tail(ctx, logTail, image.Name, "1", image.Namespace)
		require.NoError(t, err)
	}()

	t.Logf("Waiting for image '%s' to be created", image.Name)
	waitUntilReady(t, clients, image)

	registryClient := &registry.Client{}
	_, _, err = registryClient.Fetch(authn.DefaultKeychain, image.Spec.Tag)
	require.NoError(t, err)

	eventually(t, func() bool {
		return strings.Contains(logTail.String(), "Build successful")
	}, 1*time.Second, 10*time.Second)

	podList, err := clients.k8sClient.CoreV1().Pods(image.Namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("image.build.pivotal.io/image=%s", image.Name),
	})
	require.NoError(t, err)

	require.Len(t, podList.Items, 1)
	pod := podList.Items[0]

	require.Equal(t, 1, len(pod.Spec.Containers))
	assert.Equal(t, expectedResources, pod.Spec.Containers[0].Resources)
}

func validateRebase(t *testing.T, clients *clients, imageName, testNamespace string) {
	var rebaseBuildName = imageName + "-rebase"

	buildList, err := clients.client.BuildV1alpha1().Builds(testNamespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("image.build.pivotal.io/image=%s", imageName),
	})
	require.NoError(t, err)

	require.Len(t, buildList.Items, 1)
	build := buildList.Items[0]

	rebaseBuildBuildSpec := build.Spec.DeepCopy()
	rebaseBuildBuildSpec.LastBuild = &v1alpha1.LastBuild{
		Image:   build.Status.LatestImage,
		StackId: build.Status.Stack.ID,
	}

	_, err = clients.client.BuildV1alpha1().Builds(testNamespace).Create(&v1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:        rebaseBuildName,
			Annotations: map[string]string{v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonStack},
		},
		Spec: *rebaseBuildBuildSpec,
	})
	require.NoError(t, err)

	eventually(t, func() bool {
		build, err := clients.client.BuildV1alpha1().Builds(testNamespace).Get(rebaseBuildName, metav1.GetOptions{})
		require.NoError(t, err)

		require.LessOrEqual(t, len(build.Status.StepsCompleted), 1)

		return build.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsTrue()
	}, 5*time.Second, 2*time.Minute)
}

func deleteImageTag(t *testing.T, deleteImageTag string) {
	reference, err := name.ParseReference(deleteImageTag, name.WeakValidation)
	require.NoError(t, err)

	authenticator, err := authn.DefaultKeychain.Resolve(reference.Context().Registry)
	require.NoError(t, err)

	err = remote.Delete(reference, remote.WithAuth(authenticator))
	require.NoError(t, err)
}

func deleteNamespace(t *testing.T, clients *clients, namespace string) {
	err := clients.k8sClient.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
	require.True(t, err == nil || errors.IsNotFound(err))
	if errors.IsNotFound(err) {
		return
	}

	var (
		timeout int64 = 120
		closed        = false
	)

	watcher, err := clients.k8sClient.CoreV1().Namespaces().Watch(metav1.ListOptions{
		TimeoutSeconds: &timeout,
	})
	require.NoError(t, err)

	for evt := range watcher.ResultChan() {
		if evt.Type != watch.Deleted {
			continue
		}
		if ns, ok := evt.Object.(*v1.Namespace); ok {
			if ns.Name == namespace {
				closed = true
				break
			}
		}
	}
	require.True(t, closed)
}
