package test

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/buildpack/imgutil"
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
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/logs"
)

func TestCreateImage(t *testing.T) {
	spec.Run(t, "CreateImage", testCreateImage, spec.Sequential())
}

func testCreateImage(t *testing.T, when spec.G, it spec.S) {
	var cfg config
	var clients *clients
	var totalImagesCreated = 1

	const (
		testNamespace      = "test-build-service-system"
		dockerSecret       = "docker-secret"
		builderName        = "build-service-builder"
		serviceAccountName = "image-service-account"
		builderImage       = "cloudfoundry/cnb:bionic"
	)

	it.Before(func() {
		cfg = loadConfig(t)

		var err error
		clients, err = newClients()
		require.NoError(t, err)

		err = clients.k8sClient.CoreV1().Namespaces().Delete(testNamespace, &metav1.DeleteOptions{})
		require.True(t, err == nil || errors.IsNotFound(err))
		if err == nil {
			time.Sleep(10 * time.Second)
		}

		_, err = clients.k8sClient.CoreV1().Namespaces().Create(&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		})
		require.NoError(t, err)
	})

	it.After(func() {
		for i := 1; i < totalImagesCreated; i++ {
			deleteImageTag(t, cfg.imageTag+"-"+strconv.Itoa(i))
		}
	})

	when("an image is applied", func() {
		it("builds an initial image", func() {
			require.False(t, imageExists(t, cfg.imageTag+"-1")(), "image with tag 1 need to be removed")
			require.False(t, imageExists(t, cfg.imageTag+"-2")(), "image with tag 2 need to be removed")

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

			_, err = clients.client.BuildV1alpha1().Builders(testNamespace).Create(&v1alpha1.Builder{
				ObjectMeta: metav1.ObjectMeta{
					Name: builderName,
				},
				Spec: v1alpha1.BuilderSpec{
					Image: builderImage,
				},
			})
			require.NoError(t, err)

			cacheSize := resource.MustParse("1Gi")

			expectedResources := v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("1G"),
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("50m"),
					v1.ResourceMemory: resource.MustParse("512M"),
				},
			}

			imageConfigs := map[string]v1alpha1.Source{
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
			}

			for imageName, imageSource := range imageConfigs {
				_, err = clients.client.BuildV1alpha1().Images(testNamespace).Create(&v1alpha1.Image{
					ObjectMeta: metav1.ObjectMeta{
						Name: imageName,
					},
					Spec: v1alpha1.ImageSpec{
						Tag:                         cfg.imageTag + "-" + strconv.Itoa(totalImagesCreated),
						BuilderRef:                  builderName,
						ServiceAccount:              serviceAccountName,
						Source:                      imageSource,
						CacheSize:                   &cacheSize,
						DisableAdditionalImageNames: true,
						Build: v1alpha1.ImageBuild{
							Resources: expectedResources,
						},
					},
				})
				require.NoError(t, err)

				validateImageCreate(t, clients, totalImagesCreated, imageName, testNamespace, cfg, expectedResources)
				totalImagesCreated++
			}
		})
	})
}

func validateImageCreate(t *testing.T, clients *clients, numberExpectedPods int, imageName, testNamespace string, cfg config, expectedResources v1.ResourceRequirements) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logTail := &bytes.Buffer{}
	go func() {
		err := logs.NewBuildLogsClient(clients.k8sClient).Tail(ctx, logTail, imageName, "1", testNamespace)
		require.NoError(t, err)
	}()

	t.Logf("Waiting for image '%s' to be created", cfg.imageTag+"-"+strconv.Itoa(numberExpectedPods))
	eventually(t, imageExists(t, cfg.imageTag+"-"+strconv.Itoa(numberExpectedPods)), 5*time.Second, 5*time.Minute)

	assert.Contains(t, logTail.String(), fmt.Sprintf("%s - succeeded", cfg.imageTag+"-"+strconv.Itoa(numberExpectedPods)))

	podList, err := clients.k8sClient.CoreV1().Pods(testNamespace).List(metav1.ListOptions{})
	require.NoError(t, err)

	require.Len(t, podList.Items, numberExpectedPods)
	pod := podList.Items[numberExpectedPods-1]

	for i, container := range pod.Spec.InitContainers {
		if i < 2 {
			continue // skip the non-build containers
		}
		assert.Equal(t, expectedResources, container.Resources)
	}
}

func imageExists(t *testing.T, name string) func() bool {
	return func() bool {
		_, found := imageSha(t, name)
		return found
	}
}

func imageSha(t *testing.T, name string) (string, bool) {
	remoteImage, err := imgutil.NewRemoteImage(name, authn.DefaultKeychain)
	require.NoError(t, err)

	found := remoteImage.Found()
	if !found {
		return "", found
	}

	digest, err := remoteImage.Digest()
	require.NoError(t, err)

	return digest, found
}

func deleteImageTag(t *testing.T, deleteImageTag string) {
	reference, err := name.ParseReference(deleteImageTag, name.WeakValidation)
	require.NoError(t, err)

	authenticator, err := authn.DefaultKeychain.Resolve(reference.Context().Registry)
	require.NoError(t, err)

	err = remote.Delete(reference, remote.WithAuth(authenticator))
	require.NoError(t, err)
}
