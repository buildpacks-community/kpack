package test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestRebuild(t *testing.T) {
	rand.Seed(time.Now().Unix())

	ctx := context.Background()

	cfg := loadConfig(t)
	t.Logf("Using image registry: %s", cfg.testRegistry)

	clients, err := newClients(t)
	require.NoError(t, err)

	prepareTestWorkspace(t, ctx, clients, cfg)
	defer deleteCreatedImages(t, cfg)

	spec.Run(t, "Rebuild", testRebuild(ctx, cfg, clients))
}

func testRebuild(ctx context.Context, cfg config, clients *clients) func(t *testing.T, when spec.G, it spec.S) {
	return func(t *testing.T, when spec.G, it spec.S) {
		const imageResourceName = "test-rebuild-image"

		it.After(func() {
			t.Logf("Deleting image resource: %s", imageResourceName)
			err := clients.client.KpackV1alpha2().Images(testNamespace).Delete(ctx, imageResourceName, metav1.DeleteOptions{})
			if !errors.IsNotFound(err) {
				require.NoError(t, err)
			}
		})

		it("can trigger rebuilds with volume cache", func() {
			cacheSize := resource.MustParse("1Gi")

			volumeCacheConfig := &buildapi.ImageCacheConfig{
				Volume: &buildapi.ImagePersistentVolumeCache{
					Size: &cacheSize,
				},
			}

			generateRebuild(&ctx, t, cfg, clients, volumeCacheConfig, testNamespace, imageResourceName, clusterBuilderName, serviceAccountName)
		})

		it("can trigger rebuilds with registry cache", func() {
			cacheImageTag := cfg.newImageTag() + "-cache"

			registryCacheConfig := &buildapi.ImageCacheConfig{
				Registry: &buildapi.RegistryCache{
					Tag: cacheImageTag,
				},
			}
			generateRebuild(&ctx, t, cfg, clients, registryCacheConfig, testNamespace, imageResourceName, clusterBuilderName, serviceAccountName)
		})
	}
}

func generateRebuild(ctx *context.Context, t *testing.T, cfg config, clients *clients, cacheConfig *buildapi.ImageCacheConfig, testNamespace, imageResourceName, clusterBuilderName, serviceAccountName string) {
	expectedResources := corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("1G"),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("512M"),
		},
	}

	imageTag := cfg.newImageTag()
	image, err := clients.client.KpackV1alpha2().Images(testNamespace).Create(*ctx, &buildapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name: imageResourceName,
		},
		Spec: buildapi.ImageSpec{
			Tag: imageTag,
			Builder: corev1.ObjectReference{
				Kind: buildapi.ClusterBuilderKind,
				Name: clusterBuilderName,
			},
			ServiceAccount: serviceAccountName,
			Source: corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "https://github.com/cloudfoundry-samples/cf-sample-app-nodejs",
					Revision: "master",
				},
			},
			Cache:                cacheConfig,
			ImageTaggingStrategy: corev1alpha1.None,
			Build: &corev1alpha1.ImageBuild{
				Resources: expectedResources,
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	validateImageCreate(t, clients, image, expectedResources)

	list, err := clients.client.KpackV1alpha2().Builds(testNamespace).List(*ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", imageResourceName),
	})
	require.NoError(t, err)
	require.Len(t, list.Items, 1)

	build := &list.Items[0]
	build.Annotations[buildapi.BuildNeededAnnotation] = "2006-01-02 15:04:05.000000 -0700 MST m=+0.000000000"
	_, err = clients.client.KpackV1alpha2().Builds(testNamespace).Update(*ctx, build, metav1.UpdateOptions{})
	require.NoError(t, err)

	eventually(t, func() bool {
		list, err := clients.client.KpackV1alpha2().Builds(testNamespace).List(*ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("image.kpack.io/image=%s", imageResourceName),
		})
		require.NoError(t, err)
		return len(list.Items) == 2
	}, 5*time.Second, 1*time.Minute)
}
