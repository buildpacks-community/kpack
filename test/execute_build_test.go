package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/logs"
	"github.com/pivotal/kpack/pkg/registry"
)

func TestKpackE2E(t *testing.T) {
	spec.Run(t, "CreateImage", testCreateImage)
}

func testCreateImage(t *testing.T, _ spec.G, it spec.S) {
	const (
		testNamespace        = "test"
		dockerSecret         = "docker-secret"
		gitBasicSecret       = "git-basic-secret"
		gitSSHSecret         = "git-ssh-secret"
		serviceAccountName   = "image-service-account"
		clusterStoreName     = "store"
		buildpackName        = "buildpack"
		clusterBuildpackName = "cluster-buildpack"
		clusterStackName     = "stack"
		clusterLifecycleName = "lifecycle"
		builderName          = "custom-builder"
		clusterBuilderName   = "custom-cluster-builder"
	)

	var (
		cfg         config
		clients     *clients
		ctx         = context.Background()
		builtImages map[string]struct{}

		cacheSize = resource.MustParse("1Gi")

		expectedResources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("1G"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("128M"),
			},
		}

		builderConfigs = map[string]corev1.ObjectReference{
			"custom-builder": {
				Kind: buildapi.BuilderKind,
				Name: builderName,
			},
			"custom-cluster-builder": {
				Kind: buildapi.ClusterBuilderKind,
				Name: clusterBuilderName,
			},
		}
	)

	testImage := func(name string, source corev1alpha1.SourceConfig) {
		for builderType := range builderConfigs {
			imageName := fmt.Sprintf("%s-%s", name, builderType)
			builder := builderConfigs[builderType]

			t.Run(imageName, func(t *testing.T) {
				t.Parallel()

				imageTag := cfg.newImageTag()
				image, err := clients.client.KpackV1alpha2().Images(testNamespace).Create(ctx, &buildapi.Image{
					ObjectMeta: metav1.ObjectMeta{
						Name: imageName,
					},
					Spec: buildapi.ImageSpec{
						Tag:                imageTag,
						Builder:            builder,
						ServiceAccountName: serviceAccountName,
						Source:             source,
						Cache: &buildapi.ImageCacheConfig{
							Volume: &buildapi.ImagePersistentVolumeCache{
								Size: &cacheSize,
							},
						},
						ImageTaggingStrategy: corev1alpha1.None,
						Build: &buildapi.ImageBuild{
							Resources: expectedResources,
						},
					},
				}, metav1.CreateOptions{})
				require.NoError(t, err)

				builtImages[validateImageCreate(t, clients, image, expectedResources)] = struct{}{}
				validateRebase(t, ctx, clients, image.Name, testNamespace)
			})
		}
	}

	it.Before(func() {
		// register a cleanup function that dumps crds only if the test fails
		t.Cleanup(func() {
			if t.Failed() {
				dumpK8s(t, ctx, clients, testNamespace)
			}
		})

		cfg = loadConfig(t)
		builtImages = map[string]struct{}{}

		var err error
		clients, err = newClients(t)
		require.NoError(t, err)

		err = clients.client.KpackV1alpha2().ClusterStores().Delete(ctx, clusterStoreName, metav1.DeleteOptions{})
		if !errors.IsNotFound(err) {
			require.NoError(t, err)
		}

		err = clients.client.KpackV1alpha2().Buildpacks(testNamespace).Delete(ctx, buildpackName, metav1.DeleteOptions{})
		if !errors.IsNotFound(err) {
			require.NoError(t, err)
		}

		err = clients.client.KpackV1alpha2().ClusterBuildpacks().Delete(ctx, clusterBuildpackName, metav1.DeleteOptions{})
		if !errors.IsNotFound(err) {
			require.NoError(t, err)
		}

		err = clients.client.KpackV1alpha2().ClusterStacks().Delete(ctx, clusterStackName, metav1.DeleteOptions{})
		if !errors.IsNotFound(err) {
			require.NoError(t, err)
		}

		err = clients.client.KpackV1alpha2().ClusterLifecycles().Delete(ctx, clusterLifecycleName, metav1.DeleteOptions{})
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
	})

	it.After(func() {
		for tag := range builtImages {
			deleteImageTag(t, tag)
		}
	})

	it.Before(func() {
		secret, err := cfg.makeRegistrySecret(dockerSecret, testNamespace)
		require.NoError(t, err)

		_, err = clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, secret, metav1.CreateOptions{})
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
			ImagePullSecrets: []corev1.LocalObjectReference{
				{
					Name: dockerSecret,
				},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = clients.client.KpackV1alpha2().ClusterStores().Create(ctx, &buildapi.ClusterStore{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterStoreName,
			},
			Spec: buildapi.ClusterStoreSpec{
				Sources: []corev1alpha1.ImageSource{
					{Image: "gcr.io/paketo-buildpacks/bellsoft-liberica"},
					{Image: "gcr.io/paketo-buildpacks/gradle"},
					{Image: "gcr.io/paketo-buildpacks/syft"},
					{Image: "gcr.io/paketo-buildpacks/executable-jar"},
					{Image: "gcr.io/paketo-buildpacks/dist-zip"},
					{Image: "gcr.io/paketo-buildpacks/spring-boot"},
					{Image: "gcr.io/paketo-buildpacks/go"},
				},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = clients.client.KpackV1alpha2().Buildpacks(testNamespace).Create(ctx, &buildapi.Buildpack{
			ObjectMeta: metav1.ObjectMeta{
				Name: buildpackName,
			},
			Spec: buildapi.BuildpackSpec{
				ImageSource: corev1alpha1.ImageSource{
					Image: "gcr.io/paketo-buildpacks/bellsoft-liberica",
				},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = clients.client.KpackV1alpha2().ClusterBuildpacks().Create(ctx, &buildapi.ClusterBuildpack{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterBuildpackName,
			},
			Spec: buildapi.ClusterBuildpackSpec{
				ImageSource: corev1alpha1.ImageSource{
					Image: "gcr.io/paketo-buildpacks/nodejs",
				},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = clients.client.KpackV1alpha2().ClusterStacks().Create(ctx, &buildapi.ClusterStack{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterStackName,
			},
			Spec: buildapi.ClusterStackSpec{
				Id: "io.buildpacks.stacks.jammy",
				BuildImage: buildapi.ClusterStackSpecImage{
					Image: "gcr.io/paketo-buildpacks/build-jammy-base",
				},
				RunImage: buildapi.ClusterStackSpecImage{
					Image: "gcr.io/paketo-buildpacks/run-jammy-base",
				},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = clients.client.KpackV1alpha2().ClusterLifecycles().Create(ctx, &buildapi.ClusterLifecycle{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterLifecycleName,
			},
			Spec: buildapi.ClusterLifecycleSpec{
				ImageSource: corev1alpha1.ImageSource{Image: lifecycleImage},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

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
					Lifecycle: corev1.ObjectReference{
						Name: clusterLifecycleName,
						Kind: "ClusterLifecycle",
					},
					Store: corev1.ObjectReference{
						Name: clusterStoreName,
						Kind: "ClusterStore",
					},
					Order: []buildapi.BuilderOrderEntry{
						{
							Group: []buildapi.BuilderBuildpackRef{
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/go",
										},
									},
								},
							},
						},
						{
							Group: []buildapi.BuilderBuildpackRef{
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/nodejs",
										},
									},
								},
							},
						},
						{
							Group: []buildapi.BuilderBuildpackRef{
								{
									ObjectReference: corev1.ObjectReference{
										Name: buildpackName,
										Kind: "Buildpack",
									},
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/bellsoft-liberica",
										},
									},
								},
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/gradle",
										},
										Optional: true,
									},
								},
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/syft",
										},
									},
								},
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/executable-jar",
										},
									},
								},
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/dist-zip",
										},
									},
								},
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/spring-boot",
										},
									},
								},
							},
						},
					},
				},
				ServiceAccountName: serviceAccountName,
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

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
					Lifecycle: corev1.ObjectReference{
						Name: clusterLifecycleName,
						Kind: "ClusterLifecycle",
					},
					Store: corev1.ObjectReference{
						Name: clusterStoreName,
						Kind: "ClusterStore",
					},
					Order: []buildapi.BuilderOrderEntry{
						{
							Group: []buildapi.BuilderBuildpackRef{
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/go",
										},
									},
								},
							},
						},
						{
							Group: []buildapi.BuilderBuildpackRef{
								{
									ObjectReference: corev1.ObjectReference{
										Name: clusterBuildpackName,
										Kind: "ClusterBuildpack",
									},
								},
							},
						},
						{
							Group: []buildapi.BuilderBuildpackRef{
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/bellsoft-liberica",
										},
									},
								},
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/gradle",
										},
										Optional: true,
									},
								},
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/syft",
										},
									},
								},
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/executable-jar",
										},
									},
								},
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/dist-zip",
										},
									},
								},
								{
									BuildpackRef: corev1alpha1.BuildpackRef{
										BuildpackInfo: corev1alpha1.BuildpackInfo{
											Id: "paketo-buildpacks/spring-boot",
										},
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

		waitUntilCondition(t, ctx, clients, corev1alpha1.ConditionReady, builder, clusterBuilder)
		waitUntilCondition(t, ctx, clients, buildapi.ConditionUpToDate, builder, clusterBuilder)
	})

	it("builds and rebases git, blob, and registry images from unauthenticated sources", func() {
		imageSources := map[string]corev1alpha1.SourceConfig{
			"git-image": {
				Git: &corev1alpha1.Git{
					URL:      "https://github.com/cloudfoundry-samples/cf-sample-app-nodejs",
					Revision: "master",
				},
			},
			"blob-image": {
				Blob: &corev1alpha1.Blob{
					URL: "https://storage.googleapis.com/build-service/sample-apps/spring-petclinic-2.1.0.BUILD-SNAPSHOT.jar",
				},
			},
			"registry-image": {
				Registry: &corev1alpha1.Registry{
					Image: "gcr.io/cf-build-service-public/fixtures/nodejs-source@sha256:76cb2e087b6f1355caa8ed4a5eebb1ad7376e26995a8d49a570cdc10e4976e44",
				},
			},
		}

		for imageType, imageSource := range imageSources {
			testImage(imageType, imageSource)
		}
	})

	it("builds and rebases git sources from authenticated sources", func() {
		sa := &corev1.ServiceAccount{
			Secrets: []corev1.ObjectReference{},
		}

		basicSecret, basicAuthRepo := cfg.makeGitBasicAuthSecret(gitBasicSecret, testNamespace)
		if basicSecret != nil {
			_, err := clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, basicSecret, metav1.CreateOptions{})
			require.NoError(t, err)

			sa.Secrets = append(sa.Secrets, corev1.ObjectReference{
				Name: basicSecret.Name,
			})
		}

		sshSecret, sshAuthRepo := cfg.makeGitSSHAuthSecret(gitSSHSecret, testNamespace)
		if sshSecret != nil {
			_, err := clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, sshSecret, metav1.CreateOptions{})
			require.NoError(t, err)

			sa.Secrets = append(sa.Secrets, corev1.ObjectReference{
				Name: sshSecret.Name,
			})
		}

		patch, err := json.Marshal(sa)
		require.NoError(t, err)

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Patch(ctx, serviceAccountName, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
		require.NoError(t, err)

		if basicSecret != nil {
			testImage("git-basic-auth-image", corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      basicAuthRepo,
					Revision: "main",
				},
			})
		}

		if sshSecret != nil {
			testImage("git-ssh-auth-image", corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      sshAuthRepo,
					Revision: "main",
				},
			})
		}
	})

	it("can trigger rebuilds with volume cache", func() {
		cacheSize := resource.MustParse("1Gi")

		volumeCacheConfig := &buildapi.ImageCacheConfig{
			Volume: &buildapi.ImagePersistentVolumeCache{
				Size: &cacheSize,
			},
		}

		builtImages[generateRebuild(&ctx, t, cfg, clients, volumeCacheConfig, testNamespace, clusterBuilderName, serviceAccountName)] = struct{}{}
	})

	it("can trigger rebuilds with registry cache", func() {
		cacheImageTag := cfg.newImageTag() + "-cache"

		registryCacheConfig := &buildapi.ImageCacheConfig{
			Registry: &buildapi.RegistryCache{
				Tag: cacheImageTag,
			},
		}
		builtImages[generateRebuild(&ctx, t, cfg, clients, registryCacheConfig, testNamespace, clusterBuilderName, serviceAccountName)] = struct{}{}
	})
}

func generateRebuild(ctx *context.Context, t *testing.T, cfg config, clients *clients, cacheConfig *buildapi.ImageCacheConfig, testNamespace, clusterBuilderName, serviceAccountName string) string {
	expectedResources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1G"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("128M"),
		},
	}

	imageName := fmt.Sprintf("%s-%s", "test-git-image", "cluster-builder")

	imageTag := cfg.newImageTag()
	image, err := clients.client.KpackV1alpha2().Images(testNamespace).Create(*ctx, &buildapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: imageName,
		},
		Spec: buildapi.ImageSpec{
			Tag: imageTag,
			Builder: corev1.ObjectReference{
				Kind: buildapi.ClusterBuilderKind,
				Name: clusterBuilderName,
			},
			ServiceAccountName: serviceAccountName,
			Source: corev1alpha1.SourceConfig{
				SubPath: "nodejs/npm",
				Git: &corev1alpha1.Git{
					URL:      "https://github.com/paketo-buildpacks/samples",
					Revision: "becab5f3517eeb6922971ccd3e1f7adec522f0d4",
				},
			},
			Cache:                cacheConfig,
			ImageTaggingStrategy: corev1alpha1.None,
			Build: &buildapi.ImageBuild{
				Resources: expectedResources,
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	originalImageTag := validateImageCreate(t, clients, image, expectedResources)

	list, err := clients.client.KpackV1alpha2().Builds(testNamespace).List(*ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", imageName),
	})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)

	build := &list.Items[0]
	build.Annotations[buildapi.BuildNeededAnnotation] = "2006-01-02 15:04:05.000000 -0700 MST m=+0.000000000"
	_, err = clients.client.KpackV1alpha2().Builds(testNamespace).Update(*ctx, build, metav1.UpdateOptions{})
	require.NoError(t, err)

	eventually(t, func() bool {
		list, err := clients.client.KpackV1alpha2().Builds(testNamespace).List(*ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", imageName),
		})
		require.NoError(t, err)
		return len(list.Items) == 2
	}, 5*time.Second, 1*time.Minute)

	rebuiltImageTag := validateImageCreate(t, clients, image, expectedResources)
	require.Equal(t, originalImageTag, rebuiltImageTag)

	return originalImageTag
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

func waitUntilCondition(t *testing.T, ctx context.Context, clients *clients, condition corev1alpha1.ConditionType, objects ...kmeta.OwnerRefable) {
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

			return kResource.Status.GetCondition(apis.ConditionType(condition)).IsTrue()
		}, 1*time.Second, 8*time.Minute)
	}
}

func waitUntilFailed(t *testing.T, ctx context.Context, clients *clients, condition corev1alpha1.ConditionType, expectedMessage string, objects ...kmeta.OwnerRefable) {
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

			condition := kResource.Status.GetCondition(apis.ConditionType(condition))
			return condition.IsFalse() && condition.Message != "" && strings.Contains(condition.Message, expectedMessage)
		}, 1*time.Second, 8*time.Minute)
	}
}

func validateImageCreate(t *testing.T, clients *clients, image *buildapi.Image, expectedResources corev1.ResourceRequirements) string {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logTail := &bytes.Buffer{}
	go func() {
		err := logs.NewBuildLogsClient(clients.k8sClient).TailImage(ctx, logTail, image.Name, image.Namespace)
		require.NoError(t, err)
	}()

	t.Logf("Waiting for image '%s' to be created", image.Name)
	waitUntilCondition(t, ctx, clients, corev1alpha1.ConditionReady, image)

	registryClient := &registry.Client{}
	_, identifier, err := registryClient.Fetch(authn.DefaultKeychain, image.Spec.Tag)
	require.NoError(t, err)

	eventually(t, func() bool {
		return strings.Contains(logTail.String(), "Build successful")
	}, 1*time.Second, 10*time.Second)

	buildList, err := clients.client.KpackV1alpha2().Builds(image.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", image.Name),
	})
	require.NoError(t, err)

	podList, err := clients.k8sClient.CoreV1().Pods(image.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", image.Name),
	})
	require.NoError(t, err)

	require.Len(t, podList.Items, len(buildList.Items))

	return identifier
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

		//rebase and completion
		require.LessOrEqual(t, len(build.Status.StepsCompleted), 2)

		return build.Status.GetCondition(corev1alpha1.ConditionSucceeded).IsTrue()
	}, 5*time.Second, 1*time.Minute)
}

func deleteImageTag(t *testing.T, deleteImageTag string) {
	reference, err := name.ParseReference(deleteImageTag, name.WeakValidation)
	if err != nil {
		t.Logf("error cleaning up: could not parse reference: %s", err)
		return
	}

	authenticator, err := authn.DefaultKeychain.Resolve(reference.Context().Registry)
	if err != nil {
		t.Logf("error cleaning up: could not resolve keychain to delete tag: %s", err)
		return
	}

	err = remote.Delete(reference, remote.WithAuth(authenticator))
	if err != nil {
		t.Logf("error cleaning up: could not delete reference: %s", err)
		return
	}
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
