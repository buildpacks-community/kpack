package test

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	intoto "github.com/in-toto/in-toto-golang/in_toto"
	slsacommon "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/common"
	slsav1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	"github.com/sclevine/spec"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/options"
	"github.com/sigstore/cosign/v2/cmd/cosign/cli/verify"
	cosignremote "github.com/sigstore/cosign/v2/pkg/oci/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	cosigntesting "github.com/pivotal/kpack/pkg/cosign/testing"
	"github.com/pivotal/kpack/pkg/secret"
)

func TestSlsa(t *testing.T) {
	t.Cleanup(func() {
		fmt.Println("TestSlsa cleanup")
	})
	spec.Run(t, "SLSA", testSlsaBuild)
}

func testSlsaBuild(t *testing.T, when spec.G, it spec.S) {
	const (
		testNamespace            = "test-slsa"
		controllerNamespace      = "kpack"
		controllerServiceAccount = "controller"
		dockerSecret             = "docker-secret"
		serviceAccountName       = "image-service-account"
		clusterStoreName         = "store-slsa"
		buildpackName            = "buildpack"
		clusterBuildpackName     = "cluster-buildpack-slsa"
		clusterStackName         = "stack-slsa"
		builderName              = "custom-builder"
		clusterBuilderName       = "custom-cluster-builder-slsa"
		cosignSecretName         = "cosign-creds"
		cosignSecretRefFormat    = "k8s://%s/%s"
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

	when("no signing keys are present", func() {
		it("records the build details", func() {
			imageTag := cfg.newImageTag()
			image, err := clients.client.KpackV1alpha2().Images(testNamespace).Create(ctx, &buildapi.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "some-image",
				},
				Spec: buildapi.ImageSpec{
					Tag: imageTag,
					Builder: corev1.ObjectReference{
						Kind: buildapi.BuilderKind,
						Name: builderName,
					},
					ServiceAccountName: serviceAccountName,
					Source: corev1alpha1.SourceConfig{
						Git: &corev1alpha1.Git{
							URL:      "https://github.com/cloudfoundry-samples/cf-sample-app-nodejs",
							Revision: "master",
						},
					},
					ImageTaggingStrategy: corev1alpha1.None,
				},
			}, metav1.CreateOptions{})
			require.NoError(t, err)

			builtImages[validateImageCreate(t, clients, image, image.Resources())] = struct{}{}

			image, err = clients.client.KpackV1alpha2().Images(testNamespace).Get(ctx, image.Name, metav1.GetOptions{})
			require.NoError(t, err)

			verifySLSAProvenance(t, image.Status.LatestImage, image, false)
		})

		it("can read the source from git, blob, and registry images", func() {
			type row struct {
				name   string
				source corev1alpha1.SourceConfig
			}

			testImage := func(r row) {
				t.Run(r.name, func(t *testing.T) {
					t.Parallel()

					imageTag := cfg.newImageTag()
					image, err := clients.client.KpackV1alpha2().Images(testNamespace).Create(ctx, &buildapi.Image{
						ObjectMeta: metav1.ObjectMeta{
							Name: r.name,
						},
						Spec: buildapi.ImageSpec{
							Tag: imageTag,
							Builder: corev1.ObjectReference{
								Kind: buildapi.BuilderKind,
								Name: builderName,
							},
							ServiceAccountName:   serviceAccountName,
							Source:               r.source,
							ImageTaggingStrategy: corev1alpha1.None,
						},
					}, metav1.CreateOptions{})
					require.NoError(t, err)

					builtImages[validateImageCreate(t, clients, image, image.Resources())] = struct{}{}

					image, err = clients.client.KpackV1alpha2().Images(testNamespace).Get(ctx, image.Name, metav1.GetOptions{})
					require.NoError(t, err)

					verifySLSAProvenance(t, image.Status.LatestImage, image, false)
				})
			}

			table := []row{
				{
					name: "git",
					source: corev1alpha1.SourceConfig{
						Git: &corev1alpha1.Git{
							URL:      "https://github.com/cloudfoundry-samples/cf-sample-app-nodejs",
							Revision: "master",
						},
					},
				},
				{
					name: "blob",
					source: corev1alpha1.SourceConfig{
						Blob: &corev1alpha1.Blob{
							URL: "https://storage.googleapis.com/build-service/sample-apps/spring-petclinic-2.1.0.BUILD-SNAPSHOT.jar",
						},
					},
				},
				{
					name: "registry",
					source: corev1alpha1.SourceConfig{
						Registry: &corev1alpha1.Registry{
							Image: "gcr.io/cf-build-service-public/fixtures/nodejs-source@sha256:76cb2e087b6f1355caa8ed4a5eebb1ad7376e26995a8d49a570cdc10e4976e44",
						},
					},
				},
			}

			for _, r := range table {
				testImage(r)
			}
		})
	})

	// TODO(chenbh): add tests for verifying rsa/ecdsa/ed25519 keys
	when("there are signing keys", func() {
		verifyViaCosignCLI := func(digest, secretRef string) {
			cmd := verify.VerifyAttestationCommand{
				IgnoreTlog:    true,
				KeyRef:        secretRef,
				PredicateType: options.PredicateSLSA1,
			}
			err := cmd.Exec(ctx, []string{digest})
			require.NoError(t, err, "Result differs from `cosign verify-attestation`")
		}

		it("signs using builder service account keys", func() {
			cosignCredSecret := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName, testNamespace, "", map[string]string{secret.SLSASecretAnnotation: ""})
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

			imageTag := cfg.newImageTag()
			image, err := clients.client.KpackV1alpha2().Images(testNamespace).Create(ctx, &buildapi.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cosign-signing",
				},
				Spec: buildapi.ImageSpec{
					Tag: imageTag,
					Builder: corev1.ObjectReference{
						Kind: buildapi.BuilderKind,
						Name: builderName,
					},
					ServiceAccountName: serviceAccountName,
					Source: corev1alpha1.SourceConfig{
						Git: &corev1alpha1.Git{
							URL:      "https://github.com/cloudfoundry-samples/cf-sample-app-nodejs",
							Revision: "master",
						},
					},
					ImageTaggingStrategy: corev1alpha1.None,
				},
			}, metav1.CreateOptions{})
			require.NoError(t, err)

			builtImages[validateImageCreate(t, clients, image, image.Resources())] = struct{}{}

			image, err = clients.client.KpackV1alpha2().Images(testNamespace).Get(ctx, image.Name, metav1.GetOptions{})
			require.NoError(t, err)

			verifySLSAProvenance(t, image.Status.LatestImage, image, true)
			verifyViaCosignCLI(image.Status.LatestImage, fmt.Sprintf(cosignSecretRefFormat, testNamespace, cosignSecretName))
		})

		it("signs using controller service account keys", func() {
			cosignCredSecret := cosigntesting.GenerateFakeKeyPair(t, cosignSecretName, controllerNamespace, "", map[string]string{secret.SLSASecretAnnotation: ""})
			_, err := clients.k8sClient.CoreV1().Secrets(controllerNamespace).Create(ctx, &cosignCredSecret, metav1.CreateOptions{})
			require.NoError(t, err)
			defer func() {
				err = clients.k8sClient.CoreV1().Secrets(controllerNamespace).Delete(ctx, cosignCredSecret.Name, metav1.DeleteOptions{})
				require.NoError(t, err)
			}()

			serviceAccount, err := clients.k8sClient.CoreV1().ServiceAccounts(controllerNamespace).Get(ctx, controllerServiceAccount, metav1.GetOptions{})
			require.NoError(t, err)

			oldSecrets := serviceAccount.Secrets
			if serviceAccount.Secrets == nil {
				serviceAccount.Secrets = make([]corev1.ObjectReference, 0)
			}
			serviceAccount.Secrets = append(serviceAccount.Secrets, corev1.ObjectReference{Name: cosignCredSecret.Name})

			serviceAccount, err = clients.k8sClient.CoreV1().ServiceAccounts(controllerNamespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
			require.NoError(t, err)
			defer func() {
				serviceAccount.Secrets = oldSecrets
				_, err = clients.k8sClient.CoreV1().ServiceAccounts(controllerNamespace).Update(ctx, serviceAccount, metav1.UpdateOptions{})
				require.NoError(t, err)
			}()

			imageTag := cfg.newImageTag()
			image, err := clients.client.KpackV1alpha2().Images(testNamespace).Create(ctx, &buildapi.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cosign-cluster-signing",
				},
				Spec: buildapi.ImageSpec{
					Tag: imageTag,
					Builder: corev1.ObjectReference{
						Kind: buildapi.BuilderKind,
						Name: builderName,
					},
					ServiceAccountName: serviceAccountName,
					Source: corev1alpha1.SourceConfig{
						Git: &corev1alpha1.Git{
							URL:      "https://github.com/cloudfoundry-samples/cf-sample-app-nodejs",
							Revision: "master",
						},
					},
					ImageTaggingStrategy: corev1alpha1.None,
				},
			}, metav1.CreateOptions{})
			require.NoError(t, err)

			builtImages[validateImageCreate(t, clients, image, image.Resources())] = struct{}{}

			image, err = clients.client.KpackV1alpha2().Images(testNamespace).Get(ctx, image.Name, metav1.GetOptions{})
			require.NoError(t, err)

			verifySLSAProvenance(t, image.Status.LatestImage, image, true)
			verifyViaCosignCLI(image.Status.LatestImage, fmt.Sprintf(cosignSecretRefFormat, controllerNamespace, cosignSecretName))
		})
	})
}

type statement struct {
	intoto.StatementHeader
	Predicate slsav1.ProvenancePredicate `json:"predicate"`
}

func parseSLSAProvenance(t *testing.T, img v1.Image) statement {
	layers, err := img.Layers()
	require.NoError(t, err)
	require.Len(t, layers, 1, "attestation images must have exactly 1 layer")

	mt, err := layers[0].MediaType()
	require.NoError(t, err)
	require.Equal(t, types.MediaType("application/vnd.dsse.envelope.v1+json"), mt)

	reader, err := layers[0].Uncompressed()
	require.NoError(t, err)

	var envelope dsse.Envelope
	require.NoError(t, json.NewDecoder(reader).Decode(&envelope))

	require.Equal(t, "application/vnd.in-toto+json", envelope.PayloadType)

	payloadBytes, err := base64.StdEncoding.DecodeString(envelope.Payload)
	require.NoError(t, err)

	var stmt statement
	require.NoError(t, json.Unmarshal(payloadBytes, &stmt))
	return stmt
}

func verifySLSAProvenance(t *testing.T, digest string, image *buildapi.Image, signed bool) statement {
	ref, err := name.ParseReference(digest)
	require.NoError(t, err)

	auth, err := authn.DefaultKeychain.Resolve(ref.Context().Registry)
	require.NoError(t, err)

	appImg, err := remote.Image(ref, remote.WithAuth(auth))
	require.NoError(t, err)

	appDigest, err := appImg.Digest()
	require.NoError(t, err)

	attTag, err := cosignremote.AttestationTag(ref)
	require.NoError(t, err)

	attImg, err := remote.Image(attTag, remote.WithAuth(auth))
	require.NoError(t, err)

	stmt := parseSLSAProvenance(t, attImg)

	// asserts instead of requires are used so that in case we change the
	// attestation format, we consolidate all the failures in a single run
	// rather than having to rerun the test for every little typo
	assert.Equal(t, "https://slsa.dev/provenance/v1", stmt.PredicateType)

	require.Len(t, stmt.Subject, 1)
	assert.Equal(t, stmt.Subject[0], intoto.Subject{
		Name: ref.Context().Name(),
		Digest: slsacommon.DigestSet{
			appDigest.Algorithm: appDigest.Hex,
		},
	})

	pred := stmt.Predicate
	assert.Regexp(t, "^https://github.com/buildpacks-community/kpack/blob/v.*/docs/slsa.md$", pred.BuildDefinition.BuildType)
	// external params
	params, ok := pred.BuildDefinition.ExternalParameters.(map[string]interface{})
	require.True(t, ok)
	assert.NotNil(t, params["source"])
	assert.NotNil(t, params["tags"])
	assert.NotNil(t, params["runImage"])

	// internal params
	assert.Contains(t, pred.BuildDefinition.InternalParameters, "builderImage")
	assert.Contains(t, pred.BuildDefinition.InternalParameters, "completionImage")

	// build depedencies
	deps := pred.BuildDefinition.ResolvedDependencies
	require.Len(t, deps, 2)

	assert.Equal(t, deps[0].Name, "source")
	assert.Equal(t, deps[1].Name, "builder-image")
	assert.NotEmpty(t, deps[1].URI)
	assert.Contains(t, deps[1].Digest, "sha256")
	assert.Greater(t, len(deps[1].Annotations), 0)

	// builder run details
	if signed {
		assert.Equal(t, "https://kpack.io/slsa/signed-build", pred.RunDetails.Builder.ID)
	} else {
		assert.Equal(t, "https://kpack.io/slsa/unsigned-build", pred.RunDetails.Builder.ID)
	}
	assert.Contains(t, pred.RunDetails.Builder.Version, "kpack")
	assert.Contains(t, pred.RunDetails.Builder.Version, "lifecycle")
	assert.Greater(t, len(pred.RunDetails.Builder.BuilderDependencies), 0)

	// builder metadata
	metadata := pred.RunDetails.BuildMetadata
	expectedId := fmt.Sprintf("^https://kpack.io/%v/%v/.*@.*$", image.Namespace, image.Status.LatestBuildRef)
	assert.Regexp(t, expectedId, metadata.InvocationID)
	assert.NotNil(t, metadata.StartedOn)
	assert.NotNil(t, metadata.FinishedOn)

	// source metadata
	source, ok := params["source"].(map[string]interface{})
	require.True(t, ok)
	resolvedSource := deps[0]
	switch {
	case image.Spec.Source.Git != nil:
		innerSource, ok := source["git"].(map[string]interface{})
		require.True(t, ok)

		require.Equal(t, image.Spec.Source.Git.URL, innerSource["url"])
		require.NotEmpty(t, innerSource["revision"])

		require.Equal(t, image.Spec.Source.Git.URL, resolvedSource.URI)
		require.Equal(t, resolvedSource.Digest["sha1"], innerSource["revision"])

	case image.Spec.Source.Blob != nil:
		innerSource, ok := source["blob"].(map[string]interface{})
		require.True(t, ok)

		require.Equal(t, image.Spec.Source.Blob.URL, innerSource["url"])

		require.Equal(t, image.Spec.Source.Blob.URL, resolvedSource.URI)
		require.NotEmpty(t, resolvedSource.Digest["sha256"])

	case image.Spec.Source.Registry != nil:
		innerSource, ok := source["registry"].(map[string]interface{})
		require.True(t, ok)

		digest := image.Spec.Source.Registry.Image
		repo := digest[:strings.Index(digest, "@")]
		sha := digest[strings.Index(digest, ":")+1:]

		require.Equal(t, image.Spec.Source.Registry.Image, innerSource["image"])

		require.Equal(t, repo, resolvedSource.URI)
		require.Equal(t, sha, resolvedSource.Digest["sha256"])
	}

	return stmt
}

func makePrivateKey(t *testing.T, alg, secretName, namespace string) *corev1.Secret {
	t.Helper()

	var keyBytes []byte
	var (
		key any
		err error
	)
	switch alg {
	case "rsa":
		key, err = rsa.GenerateKey(rand.Reader, 1024)
		require.NoError(t, err)
	case "ecdsa":
		key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)
	case "ed25519":
		_, key, err = ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
	default:
		t.Fatal("invalid key type")
	}
	keyBytes, err = x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	pem := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})
	require.NotNil(t, pem)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
			Annotations: map[string]string{
				secret.SLSASecretAnnotation: "",
			},
		},
		Data: map[string][]byte{
			secret.PKCS8SecretKey: pem,
		},
		Type: corev1.SecretTypeSSHAuth,
	}
}
