package test

import (
	"context"
	"fmt"
	"testing"

	cosigntesting "github.com/pivotal/kpack/pkg/cosign/testing"
	"github.com/pivotal/kpack/pkg/secret"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
)

func TestSignBuilder(t *testing.T) {
	spec.Run(t, "SignBuilder", testSignBuilder)
}

func testSignBuilder(t *testing.T, _ spec.G, it spec.S) {
	const (
		testNamespace        = "test-cosign"
		dockerSecret         = "docker-secret"
		serviceAccountName   = "image-service-account"
		clusterStoreName     = "store-cosign"
		buildpackName        = "buildpack"
		clusterBuildpackName = "cluster-buildpack-cosign"
		clusterStackName     = "stack-cosign"
		clusterLifecycleName = "lifecycle-cosign"
		builderName          = "custom-signed-builder"
		clusterBuilderName   = "custom-signed-cluster-builder-cosign"
		cosignSecretName     = "cosign-creds"
		secretRefFormat      = "k8s://%s/%s"
	)

	var (
		cfg         config
		clients     *clients
		ctx         = context.Background()
		builtImages map[string]struct{}
	)

	it.Before(func() {
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

		_, err = clients.client.KpackV1alpha2().ClusterLifecycles().Create(ctx, &buildapi.ClusterLifecycle{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterLifecycleName,
			},
			Spec: buildapi.ClusterLifecycleSpec{
				ImageSource: corev1alpha1.ImageSource{Image: "buildpacksio/lifecycle"},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)
	})

	it("Signs a Builder image successfully when the key is not password-protected", func() {
		cosignCredSecret := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName, testNamespace, "", nil)
		_, err := clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &cosignCredSecret, metav1.CreateOptions{})
		require.NoError(t, err)

		serviceAccount, err := clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
		require.NoError(t, err)

		if serviceAccount.Secrets == nil {
			serviceAccount.Secrets = make([]corev1.ObjectReference, 0)
		}
		serviceAccount.Secrets = append(serviceAccount.Secrets, corev1.ObjectReference{Name: cosignCredSecret.Name})

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
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

		waitUntilCondition(t, ctx, clients, corev1alpha1.ConditionReady, builder)
		waitUntilCondition(t, ctx, clients, buildapi.ConditionUpToDate, builder)

		updatedBuilder, err := clients.client.KpackV1alpha2().Builders(testNamespace).Get(ctx, builderName, metav1.GetOptions{})
		require.NoError(t, err)

		assert.NotEmpty(t, updatedBuilder.Status.SignaturePaths)
		assert.NotNil(t, updatedBuilder.Status.SignaturePaths[0])

		err = cosigntesting.Verify(t, fmt.Sprintf(secretRefFormat, testNamespace, cosignSecretName), updatedBuilder.Status.LatestImage, nil)
		require.NoError(t, err)
	})

	it("Signs a Builder image successfully when the key is password-protected", func() {
		const CosignKeyPassword = "password"

		cosignCredSecret := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName, testNamespace, CosignKeyPassword, nil)
		_, err := clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &cosignCredSecret, metav1.CreateOptions{})
		require.NoError(t, err)

		serviceAccount, err := clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
		require.NoError(t, err)

		if serviceAccount.Secrets == nil {
			serviceAccount.Secrets = make([]corev1.ObjectReference, 0)
		}
		serviceAccount.Secrets = append(serviceAccount.Secrets, corev1.ObjectReference{Name: cosignCredSecret.Name})

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
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

		waitUntilCondition(t, ctx, clients, corev1alpha1.ConditionReady, builder)
		waitUntilCondition(t, ctx, clients, buildapi.ConditionUpToDate, builder)

		updatedBuilder, err := clients.client.KpackV1alpha2().Builders(testNamespace).Get(ctx, builderName, metav1.GetOptions{})
		require.NoError(t, err)

		assert.NotEmpty(t, updatedBuilder.Status.SignaturePaths)
		assert.NotNil(t, updatedBuilder.Status.SignaturePaths[0])

		err = cosigntesting.Verify(t, fmt.Sprintf(secretRefFormat, testNamespace, cosignSecretName), updatedBuilder.Status.LatestImage, nil)
		require.NoError(t, err)
	})

	it("Generates more than one signature for a Builder image successfully when multiple secrets are present", func() {
		const CosignKeyPassword = "password"
		const cosignSecretName1 = "cosign-credentials-1"
		const cosignSecretName2 = "cosign-credentials-2"

		cosignCredSecret1 := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName1, testNamespace, CosignKeyPassword, nil)
		_, err := clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &cosignCredSecret1, metav1.CreateOptions{})
		require.NoError(t, err)

		cosignCredSecret2 := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName2, testNamespace, CosignKeyPassword, nil)
		_, err = clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &cosignCredSecret2, metav1.CreateOptions{})
		require.NoError(t, err)

		serviceAccount, err := clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
		require.NoError(t, err)

		if serviceAccount.Secrets == nil {
			serviceAccount.Secrets = make([]corev1.ObjectReference, 0)
		}
		serviceAccount.Secrets = append(serviceAccount.Secrets,
			corev1.ObjectReference{Name: cosignCredSecret1.Name},
			corev1.ObjectReference{Name: cosignCredSecret2.Name})

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
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

		waitUntilCondition(t, ctx, clients, corev1alpha1.ConditionReady, builder)
		waitUntilCondition(t, ctx, clients, buildapi.ConditionUpToDate, builder)

		updatedBuilder, err := clients.client.KpackV1alpha2().Builders(testNamespace).Get(ctx, builderName, metav1.GetOptions{})
		require.NoError(t, err)

		assert.NotEmpty(t, updatedBuilder.Status.SignaturePaths)
		assert.Equal(t, 2, len(updatedBuilder.Status.SignaturePaths))
		assert.NotNil(t, updatedBuilder.Status.SignaturePaths[0])
		assert.NotNil(t, updatedBuilder.Status.SignaturePaths[1])

		// tag is assigned to a single signature, but both are still verifiable
		err = cosigntesting.Verify(t, fmt.Sprintf(secretRefFormat, testNamespace, cosignSecretName1), updatedBuilder.Status.LatestImage, nil)
		require.NoError(t, err)

		err = cosigntesting.Verify(t, fmt.Sprintf(secretRefFormat, testNamespace, cosignSecretName2), updatedBuilder.Status.LatestImage, nil)
		require.NoError(t, err)
	})

	it("Saves a failure in the Builder record when signing fails", func() {
		const CosignKeyPassword = "password"
		const invalidPassword = "wrong-password"
		const expectedErrorMessage = "unable to sign"

		cosignCredSecret := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName, testNamespace, CosignKeyPassword, nil)
		cosignCredSecret.Data[secret.CosignSecretPassword] = []byte(invalidPassword)

		_, err := clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &cosignCredSecret, metav1.CreateOptions{})
		require.NoError(t, err)

		serviceAccount, err := clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
		require.NoError(t, err)

		if serviceAccount.Secrets == nil {
			serviceAccount.Secrets = make([]corev1.ObjectReference, 0)
		}
		serviceAccount.Secrets = append(serviceAccount.Secrets,
			corev1.ObjectReference{Name: cosignCredSecret.Name})

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
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

		waitUntilFailed(t, ctx, clients, buildapi.ConditionUpToDate, expectedErrorMessage, builder)
		waitUntilFailed(t, ctx, clients, corev1alpha1.ConditionReady, buildapi.NoLatestImageMessage, builder)

		updatedBuilder, err := clients.client.KpackV1alpha2().Builders(testNamespace).Get(ctx, builderName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, updatedBuilder.Status)

		readyConditionBuilder := updatedBuilder.Status.GetCondition(corev1alpha1.ConditionReady)
		require.False(t, readyConditionBuilder.IsTrue())

		upToDateConditionBuilder := updatedBuilder.Status.GetCondition(buildapi.ConditionUpToDate)
		require.False(t, upToDateConditionBuilder.IsTrue())
		require.Contains(t, upToDateConditionBuilder.Message, expectedErrorMessage)
	})

	it("Signs a ClusterBuilder image successfully when the key is not password-protected", func() {
		cosignCredSecret := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName, testNamespace, "", nil)
		_, err := clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &cosignCredSecret, metav1.CreateOptions{})
		require.NoError(t, err)

		serviceAccount, err := clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
		require.NoError(t, err)

		if serviceAccount.Secrets == nil {
			serviceAccount.Secrets = make([]corev1.ObjectReference, 0)
		}
		serviceAccount.Secrets = append(serviceAccount.Secrets, corev1.ObjectReference{Name: cosignCredSecret.Name})

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
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

		waitUntilCondition(t, ctx, clients, corev1alpha1.ConditionReady, clusterBuilder)
		waitUntilCondition(t, ctx, clients, buildapi.ConditionUpToDate, clusterBuilder)

		updatedBuilder, err := clients.client.KpackV1alpha2().ClusterBuilders().Get(ctx, clusterBuilderName, metav1.GetOptions{})
		require.NoError(t, err)

		assert.NotEmpty(t, updatedBuilder.Status.SignaturePaths)
		assert.NotNil(t, updatedBuilder.Status.SignaturePaths[0])

		err = cosigntesting.Verify(t, fmt.Sprintf(secretRefFormat, testNamespace, cosignSecretName), updatedBuilder.Status.LatestImage, nil)
		require.NoError(t, err)
	})

	it("Signs a ClusterBuilder image successfully when the key is password-protected", func() {
		const CosignKeyPassword = "password"

		cosignCredSecret := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName, testNamespace, CosignKeyPassword, nil)
		_, err := clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &cosignCredSecret, metav1.CreateOptions{})
		require.NoError(t, err)

		serviceAccount, err := clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
		require.NoError(t, err)

		if serviceAccount.Secrets == nil {
			serviceAccount.Secrets = make([]corev1.ObjectReference, 0)
		}
		serviceAccount.Secrets = append(serviceAccount.Secrets, corev1.ObjectReference{Name: cosignCredSecret.Name})

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
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

		waitUntilCondition(t, ctx, clients, corev1alpha1.ConditionReady, clusterBuilder)
		waitUntilCondition(t, ctx, clients, buildapi.ConditionUpToDate, clusterBuilder)

		updatedBuilder, err := clients.client.KpackV1alpha2().ClusterBuilders().Get(ctx, clusterBuilderName, metav1.GetOptions{})
		require.NoError(t, err)

		assert.NotEmpty(t, updatedBuilder.Status.SignaturePaths)
		assert.NotNil(t, updatedBuilder.Status.SignaturePaths[0])

		err = cosigntesting.Verify(t, fmt.Sprintf(secretRefFormat, testNamespace, cosignSecretName), updatedBuilder.Status.LatestImage, nil)
		require.NoError(t, err)
	})

	it("Generates more than one signature for a ClusterBuilder image successfully when multiple secrets are present", func() {
		const cosignKeyPassword = "password"
		const cosignSecretName1 = "cosign-credentials-1"
		const cosignSecretName2 = "cosign-credentials-2"

		cosignCredSecret1 := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName1, testNamespace, cosignKeyPassword, nil)
		_, err := clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &cosignCredSecret1, metav1.CreateOptions{})
		require.NoError(t, err)

		cosignCredSecret2 := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName2, testNamespace, cosignKeyPassword, nil)
		_, err = clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &cosignCredSecret2, metav1.CreateOptions{})
		require.NoError(t, err)

		serviceAccount, err := clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
		require.NoError(t, err)

		if serviceAccount.Secrets == nil {
			serviceAccount.Secrets = make([]corev1.ObjectReference, 0)
		}
		serviceAccount.Secrets = append(
			serviceAccount.Secrets,
			corev1.ObjectReference{Name: cosignCredSecret1.Name},
			corev1.ObjectReference{Name: cosignCredSecret2.Name})

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
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

		waitUntilCondition(t, ctx, clients, corev1alpha1.ConditionReady, clusterBuilder)
		waitUntilCondition(t, ctx, clients, buildapi.ConditionUpToDate, clusterBuilder)

		updatedBuilder, err := clients.client.KpackV1alpha2().ClusterBuilders().Get(ctx, clusterBuilderName, metav1.GetOptions{})
		require.NoError(t, err)

		assert.NotEmpty(t, updatedBuilder.Status.SignaturePaths)
		assert.Equal(t, 2, len(updatedBuilder.Status.SignaturePaths))
		assert.NotNil(t, updatedBuilder.Status.SignaturePaths[0])
		assert.NotNil(t, updatedBuilder.Status.SignaturePaths[1])

		err = cosigntesting.Verify(t, fmt.Sprintf(secretRefFormat, testNamespace, cosignSecretName1), updatedBuilder.Status.LatestImage, nil)
		require.NoError(t, err)

		err = cosigntesting.Verify(t, fmt.Sprintf(secretRefFormat, testNamespace, cosignSecretName2), updatedBuilder.Status.LatestImage, nil)
		require.NoError(t, err)
	})

	it("Saves a failure in the ClusterBuilder record when signing fails", func() {
		const cosignKeyPassword = "password"
		const invalidPassword = "wrong-password"
		const expectedErrorMessage = "unable to sign"

		cosignCredSecret := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName, testNamespace, cosignKeyPassword, nil)
		cosignCredSecret.Data[secret.CosignSecretPassword] = []byte(invalidPassword)

		_, err := clients.k8sClient.CoreV1().Secrets(testNamespace).Create(ctx, &cosignCredSecret, metav1.CreateOptions{})
		require.NoError(t, err)

		serviceAccount, err := clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Get(ctx, serviceAccountName, metav1.GetOptions{})
		require.NoError(t, err)

		if serviceAccount.Secrets == nil {
			serviceAccount.Secrets = make([]corev1.ObjectReference, 0)
		}
		serviceAccount.Secrets = append(
			serviceAccount.Secrets,
			corev1.ObjectReference{Name: cosignCredSecret.Name})

		_, err = clients.k8sClient.CoreV1().ServiceAccounts(testNamespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
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

		waitUntilFailed(t, ctx, clients, buildapi.ConditionUpToDate, expectedErrorMessage, clusterBuilder)
		waitUntilFailed(t, ctx, clients, corev1alpha1.ConditionReady, buildapi.NoLatestImageMessage, clusterBuilder)

		updatedBuilder, err := clients.client.KpackV1alpha2().ClusterBuilders().Get(ctx, clusterBuilderName, metav1.GetOptions{})
		require.NoError(t, err)
		require.NotNil(t, updatedBuilder.Status)

		readyConditionBuilder := updatedBuilder.Status.GetCondition(corev1alpha1.ConditionReady)
		require.False(t, readyConditionBuilder.IsTrue())

		upToDateConditionBuilder := updatedBuilder.Status.GetCondition(buildapi.ConditionUpToDate)
		require.False(t, upToDateConditionBuilder.IsTrue())
		require.Contains(t, upToDateConditionBuilder.Message, expectedErrorMessage)
	})
}
