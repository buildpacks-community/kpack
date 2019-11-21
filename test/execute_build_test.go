package test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/logs"
	"github.com/pivotal/kpack/pkg/registry"
)

func TestCreateImage(t *testing.T) {
	spec.Run(t, "CreateImage", testCreateImage, spec.Sequential())
}

func testCreateImage(t *testing.T, when spec.G, it spec.S) {
	var cfg config
	var clients *clients

	const (
		testNamespace      = "test"
		dockerSecret       = "docker-secret"
		builderName        = "build-service-builder"
		clusterBuilderName = "cluster-builder"
		serviceAccountName = "image-service-account"
		builderImage       = "cloudfoundry/cnb:bionic"
	)

	it.Before(func() {
		cfg = loadConfig(t)

		var err error
		clients, err = newClients(t)
		require.NoError(t, err)

		err = clients.client.BuildV1alpha1().ClusterBuilders().Delete(clusterBuilderName, &metav1.DeleteOptions{})
		require.True(t, err == nil || errors.IsNotFound(err))

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

		username, password, ok := parseBasicAuth(basicAuth)
		require.True(t, ok)

		_, err = clients.k8sClient.CoreV1().Secrets(testNamespace).Create(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: dockerSecret,
				Annotations: map[string]string{
					"build.pivotal.io/docker": reference.Context().RegistryStr(),
				},
			},
			StringData: map[string]string{
				"username": username,
				"password": password,
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

		_, err = clients.client.BuildV1alpha1().ClusterBuilders().Create(&v1alpha1.ClusterBuilder{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterBuilderName,
			},
			Spec: v1alpha1.BuilderSpec{
				Image: builderImage,
			},
		})
		require.NoError(t, err)

		_, err = clients.client.BuildV1alpha1().Builders(testNamespace).Create(&v1alpha1.Builder{
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

		cacheSize := resource.MustParse("1Gi")

		expectedResources := v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("1G"),
			},
			Requests: v1.ResourceList{
				v1.ResourceMemory: resource.MustParse("512M"),
			},
		}

		imageConfigs := map[string]v1alpha1.SourceConfig{
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
					Image: "gcr.io/cf-build-service-public/testing/beam/source@sha256:7d8aa6c87fc659d52bf42aadf23e0aaa15b1d7ed8e41383a201edabfe9d17949",
				},
			},
		}

		imageBuilders := map[string]v1alpha1.ImageBuilder{
			"test-git-image": {
				TypeMeta: metav1.TypeMeta{
					Kind:       v1alpha1.ClusterBuilderKind,
					APIVersion: "build.pivotal.io/v1alpha1",
				},
				Name: clusterBuilderName,
			},
			"test-registry-image": {
				TypeMeta: metav1.TypeMeta{
					Kind:       v1alpha1.BuilderKind,
					APIVersion: "build.pivotal.io/v1alpha1",
				},
				Name: builderName,
			},
			"test-blob-image": {
				TypeMeta: metav1.TypeMeta{
					Kind:       v1alpha1.ClusterBuilderKind,
					APIVersion: "build.pivotal.io/v1alpha1",
				},
				Name: clusterBuilderName,
			},
		}

		for imageName, imageSource := range imageConfigs {
			imageTag := cfg.newImageTag()
			_, err := clients.client.BuildV1alpha1().Images(testNamespace).Create(&v1alpha1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: imageName,
				},
				Spec: v1alpha1.ImageSpec{
					Tag:                  imageTag,
					Builder:              imageBuilders[imageName],
					ServiceAccount:       serviceAccountName,
					Source:               imageSource,
					CacheSize:            &cacheSize,
					ImageTaggingStrategy: v1alpha1.None,
					Build: &v1alpha1.ImageBuild{
						Resources: expectedResources,
					},
				},
			})
			require.NoError(t, err)

			validateImageCreate(t, clients, imageTag, imageName, testNamespace, expectedResources)

			validateRebase(t, clients, imageName, testNamespace)
		}
	})
}

func validateImageCreate(t *testing.T, clients *clients, imageTag, imageName, testNamespace string, expectedResources v1.ResourceRequirements) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logTail := &bytes.Buffer{}
	go func() {
		err := logs.NewBuildLogsClient(clients.k8sClient).Tail(ctx, logTail, imageName, "1", testNamespace)
		require.NoError(t, err)
	}()

	t.Logf("Waiting for image '%s' to be created", imageTag)
	eventually(t, imageExists(imageTag), 5*time.Second, 5*time.Minute)

	eventually(t, func() bool {
		return strings.Contains(logTail.String(), "Build successful")
	}, 5*time.Second, 1*time.Minute)

	podList, err := clients.k8sClient.CoreV1().Pods(testNamespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("image.build.pivotal.io/image=%s", imageName),
	})
	require.NoError(t, err)

	require.Len(t, podList.Items, 1)
	pod := podList.Items[0]

	require.Equal(t, 1, len(pod.Spec.Containers))
	assert.Equal(t, expectedResources, pod.Spec.Containers[0].Resources)
}

func imageExists(name string) func() bool {
	return func() bool {
		_, err := registry.NewGoContainerRegistryImage(name, authn.DefaultKeychain)
		if err != nil {
			return false
		}
		return true
	}
}

func validateRebase(t *testing.T, clients *clients, imageName, testNamespace string) {
	var build v1alpha1.Build

	eventually(t, func() bool {
		buildList, err := clients.client.BuildV1alpha1().Builds(testNamespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("image.build.pivotal.io/image=%s", imageName),
		})
		require.NoError(t, err)

		require.Len(t, buildList.Items, 1)
		build = buildList.Items[0]

		return build.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsTrue()
	}, 5*time.Second, 1*time.Minute)

	rebaseBuildBuildSpec := build.Spec.DeepCopy()
	rebaseBuildBuildSpec.LastBuild = &v1alpha1.LastBuild{
		Image:   build.Status.LatestImage,
		StackID: build.Status.Stack.ID,
	}

	_, err = clients.client.BuildV1alpha1().Builds(testNamespace).Create(&v1alpha1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:        build.Name + "-rebase",
			Annotations: map[string]string{v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonStack},
		},
		Spec: *rebaseBuildBuildSpec,
	})
	require.NoError(t, err)

	eventually(t, func() bool {
		build, err := clients.client.BuildV1alpha1().Builds(testNamespace).Get(build.Name+"-rebase", metav1.GetOptions{})
		require.NoError(t, err)
		return build.Status.GetCondition(duckv1alpha1.ConditionSucceeded).IsTrue() && len(build.Status.StepsCompleted) == 1
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
