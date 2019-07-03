package test

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	"github.com/buildpack/imgutil"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
)

func TestCreateImage(t *testing.T) {
	spec.Run(t, "CreateImage", testCreateImage, spec.Sequential())
}

func testCreateImage(t *testing.T, when spec.G, it spec.S) {
	var cfg config
	var clients *clients

	const (
		testNamespace      = "test-build-service-system"
		dockerSecret       = "docker-secret"
		imageName          = "test-image"
		builderName        = "build-service-builder"
		serviceAccountName = "image-service-account"
		builderImage       = "cloudfoundry/cnb:bionic"
	)

	it.Before(func() {
		cfg = loadConfig(t)

		var err error
		clients, err = newClients()
		require.NoError(t, err)

		err = clients.k8sClient.Namespaces().Delete(testNamespace, &metav1.DeleteOptions{})
		require.True(t, err == nil || errors.IsNotFound(err))
		if err == nil {
			time.Sleep(10 * time.Second)
		}

		_, err = clients.k8sClient.Namespaces().Create(&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		})
		require.NoError(t, err)
	})

	it.After(func() {
		deleteImageTag(t, cfg.imageTag)
	})

	when("an image is applied", func() {
		it("builds an initial image", func() {
			require.False(t, imageExists(t, cfg.imageTag)())

			reference, err := name.ParseReference(cfg.imageTag, name.WeakValidation)
			require.NoError(t, err)
			auth, err := authn.DefaultKeychain.Resolve(reference.Context().Registry)
			require.NoError(t, err)

			basicAuth, err := auth.Authorization()
			require.NoError(t, err)

			username, password, ok := parseBasicAuth(basicAuth)
			require.True(t, ok)

			_, err = clients.k8sClient.Secrets(testNamespace).Create(&v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: dockerSecret,
					Annotations: map[string]string{
						"build.knative.dev/docker-0": reference.Context().RegistryStr(),
					},
				},
				StringData: map[string]string{
					"username": username,
					"password": password,
				},
				Type: v1.SecretTypeBasicAuth,
			})
			require.NoError(t, err)

			_, err = clients.k8sClient.ServiceAccounts(testNamespace).Create(&v1.ServiceAccount{
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
			_, err = clients.client.BuildV1alpha1().Images(testNamespace).Create(&v1alpha1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: imageName,
				},
				Spec: v1alpha1.ImageSpec{
					Image:          cfg.imageTag,
					BuilderRef:     builderName,
					ServiceAccount: serviceAccountName,
					Source: v1alpha1.Source{
						Git: v1alpha1.Git{
							URL:      "https://github.com/cloudfoundry-samples/cf-sample-app-nodejs",
							Revision: "master",
						},
					},
					CacheSize: &cacheSize,
				},
			})
			require.NoError(t, err)

			t.Logf("Waiting for image '%s' to be created", cfg.imageTag)
			eventually(t, imageExists(t, cfg.imageTag), 5*time.Second, 5*time.Minute)
		})
	})
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
