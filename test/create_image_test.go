package test

import (
	"context"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestCreateImage(t *testing.T) {
	rand.Seed(time.Now().Unix())

	ctx := context.Background()

	cfg := loadConfig(t)
	t.Logf("Using image registry: %s", cfg.testRegistry)

	clients, err := newClients(t)
	require.NoError(t, err)

	prepareTestWorkspace(t, ctx, clients, cfg)
	defer deleteCreatedImages(t, cfg)

	spec.Run(t, "CreateImage", testCreateImage(ctx, cfg, clients))
}

func testCreateImage(ctx context.Context, cfg config, clients *clients) func(t *testing.T, when spec.G, it spec.S) {
	return func(t *testing.T, when spec.G, it spec.S) {
		when("builds and rebases git, blob, and registry based images", func() {
			cacheSize := resource.MustParse("1Gi")

			expectedResources := corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1G"),
				},
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512M"),
				},
			}

			imageSources := map[string]corev1alpha1.SourceConfig{
				"test-git-image": {
					Git: &corev1alpha1.Git{
						URL:      "https://github.com/cloudfoundry-samples/cf-sample-app-nodejs",
						Revision: "master",
					},
				},
				"test-blob-image": {
					Blob: &corev1alpha1.Blob{
						URL: "https://storage.googleapis.com/build-service/sample-apps/spring-petclinic-2.1.0.BUILD-SNAPSHOT.jar",
					},
				},
				"test-registry-image": {
					Registry: &corev1alpha1.Registry{
						Image: "gcr.io/cf-build-service-public/fixtures/nodejs-source@sha256:76cb2e087b6f1355caa8ed4a5eebb1ad7376e26995a8d49a570cdc10e4976e44",
					},
				},
			}

			builderConfigs := map[string]corev1.ObjectReference{
				"custom-builder": {
					Kind: buildapi.BuilderKind,
					Name: builderName,
				},
				"custom-cluster-builder": {
					Kind: buildapi.ClusterBuilderKind,
					Name: clusterBuilderName,
				},
			}

			imageTypes := make([]string, 0, len(imageSources))
			for k := range imageSources {
				imageTypes = append(imageTypes, k)
			}
			sort.Strings(imageTypes)

			builderTypes := make([]string, 0, len(builderConfigs))
			for k := range builderConfigs {
				builderTypes = append(builderTypes, k)
			}
			sort.Strings(builderTypes)

			for _, imageType := range imageTypes {
				for _, builderType := range builderTypes {
					imageName := imageType + "-" + builderType
					source := imageSources[imageType]
					builder := builderConfigs[builderType]

					it(imageName, func() {
						imageTag := cfg.newImageTag()
						image, err := clients.client.KpackV1alpha2().Images(testNamespace).Create(ctx,
							&buildapi.Image{
								ObjectMeta: metav1.ObjectMeta{
									Name: imageName,
								},
								Spec: buildapi.ImageSpec{
									Tag:            imageTag,
									Builder:        builder,
									ServiceAccount: serviceAccountName,
									Source:         source,
									Cache: &buildapi.ImageCacheConfig{
										Volume: &buildapi.ImagePersistentVolumeCache{
											Size: &cacheSize,
										},
									},
									ImageTaggingStrategy: corev1alpha1.None,
									Build: &corev1alpha1.ImageBuild{
										Resources: expectedResources,
									},
								},
							}, metav1.CreateOptions{})

						require.NoError(t, err)

						validateImageCreate(t, clients, image, expectedResources)
						validateRebase(t, ctx, clients, image.Name, testNamespace)
					}, spec.Parallel())
				}
			}
		})
	}
}
