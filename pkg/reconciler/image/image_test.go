package image_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
	rtesting "knative.dev/pkg/reconciler/testing"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/reconciler/image"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestImageReconciler(t *testing.T) {
	spec.Run(t, "Image Reconciler", testImageReconciler)
}

func testImageReconciler(t *testing.T, when spec.G, it spec.S) {

	const (
		imageName                    = "image-name"
		builderName                  = "builder-name"
		clusterBuilderName           = "cluster-builder-name"
		serviceAccount               = "service-account"
		namespace                    = "some-namespace"
		key                          = "some-namespace/image-name"
		someLabelKey                 = "some/label"
		someValueToPassThrough       = "to-pass-through"
		originalGeneration     int64 = 1
	)
	var (
		fakeTracker = testhelpers.FakeTracker{}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)

			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			k8sfakeClient := k8sfake.NewSimpleClientset(listers.GetKubeObjects()...)

			eventRecorder := record.NewFakeRecorder(10)
			actionRecorderList := rtesting.ActionRecorderList{fakeClient, k8sfakeClient}
			eventList := rtesting.EventList{Recorder: eventRecorder}

			r := &image.Reconciler{
				Client:               fakeClient,
				ImageLister:          listers.GetImageLister(),
				BuildLister:          listers.GetBuildLister(),
				DuckBuilderLister:    listers.GetDuckBuilderLister(),
				SourceResolverLister: listers.GetSourceResolverLister(),
				PvcLister:            listers.GetPersistentVolumeClaimLister(),
				Tracker:              fakeTracker,
				K8sClient:            k8sfakeClient,
			}

			rtesting.PrependGenerateNameReactor(&fakeClient.Fake)

			return r, actionRecorderList, eventList
		})

	image := &buildapi.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:       imageName,
			Namespace:  namespace,
			Generation: originalGeneration,
			Labels: map[string]string{
				someLabelKey: someValueToPassThrough,
			},
		},
		Spec: buildapi.ImageSpec{
			Tag: "some/image",
			Builder: corev1.ObjectReference{
				Kind: "Builder",
				Name: builderName,
			},
			ServiceAccount: serviceAccount,
			Source: corev1alpha1.SourceConfig{
				Git: &corev1alpha1.Git{
					URL:      "https://some.git/url",
					Revision: "1234567",
				},
			},
			FailedBuildHistoryLimit:  limit(10),
			SuccessBuildHistoryLimit: limit(10),
			ImageTaggingStrategy:     corev1alpha1.None,
			Build:                    &buildapi.ImageBuild{},
		},
		Status: buildapi.ImageStatus{
			Status: corev1alpha1.Status{
				ObservedGeneration: originalGeneration,
				Conditions:         conditionReadyUnknown(),
			},
		},
	}

	builder := &buildapi.Builder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builderName,
			Namespace: namespace,
		},
		Status: buildapi.BuilderStatus{
			LatestImage: "some/builder@sha256:acf123",
			BuilderMetadata: corev1alpha1.BuildpackMetadataList{
				{
					Id:      "buildpack.version",
					Version: "version",
				},
			},
			Stack: corev1alpha1.BuildStack{
				RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
				ID:       "io.buildpacks.stacks.bionic",
			},
			Status: corev1alpha1.Status{
				Conditions: corev1alpha1.Conditions{
					{
						Type:   corev1alpha1.ConditionReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	clusterBuilder := &buildapi.ClusterBuilder{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterBuilderName,
		},
		Status: buildapi.BuilderStatus{
			LatestImage: "some/clusterbuilder@sha256:acf123",
			BuilderMetadata: corev1alpha1.BuildpackMetadataList{
				{
					Id:      "buildpack.version",
					Version: "version",
				},
			},
			Stack: corev1alpha1.BuildStack{
				RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
				ID:       "io.buildpacks.stacks.bionic",
			},

			Status: corev1alpha1.Status{
				Conditions: corev1alpha1.Conditions{
					{
						Type:   corev1alpha1.ConditionReady,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	when("Reconcile", func() {
		it("updates observed generation after processing an update", func() {
			const updatedGeneration int64 = 2
			image.ObjectMeta.Generation = updatedGeneration

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					image,
					builder,
					clusterBuilder,
					unresolvedSourceResolver(image),
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Image{
							ObjectMeta: image.ObjectMeta,
							Spec:       image.Spec,
							Status: buildapi.ImageStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: updatedGeneration,
									Conditions:         conditionReadyUnknown(),
								},
							},
						},
					},
				},
			})
		})

		it("does not update status if there is no status update", func() {
			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					image,
					builder,
					clusterBuilder,
					unresolvedSourceResolver(image),
				},
				WantErr: false,
			})
		})

		it("tracks builder for image", func() {
			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					image,
					builder,
					clusterBuilder,
					unresolvedSourceResolver(image),
				},
				WantErr: false,
			})

			require.True(t, fakeTracker.IsTracking(builder, image.NamespacedName()))
		})

		it("sets condition not ready for non-existent builder", func() {
			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					image,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Image{
							ObjectMeta: image.ObjectMeta,
							Spec:       image.Spec,
							Status: buildapi.ImageStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: corev1alpha1.Conditions{
										{
											Type:    corev1alpha1.ConditionReady,
											Status:  corev1.ConditionFalse,
											Reason:  "BuilderNotFound",
											Message: "Unable to find builder builder-name.",
										},
									},
								},
							},
						},
					},
				},
			})
		})

		when("reconciling source resolvers", func() {
			it("creates a source resolver if not created", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.SourceResolver{
							ObjectMeta: metav1.ObjectMeta{
								Name:      image.SourceResolverName(),
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									someLabelKey: someValueToPassThrough,
								},
							},
							Spec: buildapi.SourceResolverSpec{
								ServiceAccount: image.Spec.ServiceAccount,
								Source:         image.Spec.Source,
							},
						},
					},
				})
			})

			it("does not create a source resolver if already created", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						image.SourceResolver(),
					},
					WantErr: false,
				})
			})

			it("updates source resolver if configuration changed", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						&buildapi.SourceResolver{
							ObjectMeta: metav1.ObjectMeta{
								Name:      image.SourceResolverName(),
								Namespace: namespace,
								Labels: map[string]string{
									someLabelKey: someValueToPassThrough,
								},
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
							},
							Spec: buildapi.SourceResolverSpec{
								ServiceAccount: "old-account",
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      "old-url",
										Revision: "old-revision",
									},
								},
							},
						},
					},
					WantErr: false,
					WantUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.SourceResolver{
								ObjectMeta: metav1.ObjectMeta{
									Name:      image.SourceResolverName(),
									Namespace: namespace,
									Labels: map[string]string{
										someLabelKey: someValueToPassThrough,
									},
									OwnerReferences: []metav1.OwnerReference{
										*kmeta.NewControllerRef(image),
									},
								},
								Spec: buildapi.SourceResolverSpec{
									ServiceAccount: image.Spec.ServiceAccount,
									Source:         image.Spec.Source,
								},
							},
						},
					},
				})
			})

			it("updates source resolver if labels change", func() {
				sourceResolver := image.SourceResolver()

				extraLabelImage := image.DeepCopy()
				extraLabelImage.Labels["another/label"] = "label"
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						extraLabelImage,
						builder,
						sourceResolver,
					},
					WantErr: false,
					WantUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.SourceResolver{
								ObjectMeta: metav1.ObjectMeta{
									Name:      image.SourceResolverName(),
									Namespace: namespace,
									OwnerReferences: []metav1.OwnerReference{
										*kmeta.NewControllerRef(image),
									},
									Labels: map[string]string{
										someLabelKey:    someValueToPassThrough,
										"another/label": "label",
									},
								},
								Spec: buildapi.SourceResolverSpec{
									ServiceAccount: image.Spec.ServiceAccount,
									Source:         image.Spec.Source,
								},
							},
						},
					},
				})
			})
		})

		when("reconciling build caches", func() {
			cacheSize := resource.MustParse("1.5")
			image.Spec.Cache = &buildapi.ImageCacheConfig{
				Volume: &buildapi.ImagePersistentVolumeCache{
					Size: &cacheSize,
				},
			}

			it("creates a cache if requested", func() {

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						image.SourceResolver(),
						builder,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      image.CacheName(),
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									someLabelKey: someValueToPassThrough,
								},
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: cacheSize,
									},
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									BuildCacheName: image.CacheName(),
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionReadyUnknown(),
									},
								},
							},
						},
					},
				})
			})

			it("does not create a cache if a cache already exists", func() {
				image.Spec.Cache.Volume.Size = &cacheSize
				image.Status.BuildCacheName = image.CacheName()

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						image.SourceResolver(),
						image.BuildCache(),
						builder,
					},
					WantErr: false,
				})
			})

			it("updates build cache if desired configuration changed", func() {
				var imageCacheName = image.CacheName()

				image.Status.BuildCacheName = imageCacheName
				newCacheSize := resource.MustParse("2.5")
				image.Spec.Cache.Volume.Size = &newCacheSize

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						image.SourceResolver(),
						builder,
						&corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageCacheName,
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: cacheSize,
									},
								},
							},
						},
					},
					WantErr: false,
					WantUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &corev1.PersistentVolumeClaim{
								ObjectMeta: metav1.ObjectMeta{
									Name:      imageCacheName,
									Namespace: namespace,
									Labels: map[string]string{
										someLabelKey: someValueToPassThrough,
									},
									OwnerReferences: []metav1.OwnerReference{
										*kmeta.NewControllerRef(image),
									},
								},
								Spec: corev1.PersistentVolumeClaimSpec{
									AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: newCacheSize,
										},
									},
								},
							},
						},
					},
				})
			})

			it("updates build cache if desired labels change", func() {
				var imageCacheName = image.CacheName()
				image.Spec.Cache.Volume.Size = &cacheSize
				image.Status.BuildCacheName = imageCacheName
				cache := image.BuildCache()

				extraLabelImage := image.DeepCopy()
				extraLabelImage.Labels["another/label"] = "label"
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						extraLabelImage,
						extraLabelImage.SourceResolver(),
						builder,
						cache,
					},
					WantErr: false,
					WantUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &corev1.PersistentVolumeClaim{
								ObjectMeta: metav1.ObjectMeta{
									Name: imageCacheName,
									OwnerReferences: []metav1.OwnerReference{
										*kmeta.NewControllerRef(image),
									},
									Namespace: namespace,
									Labels: map[string]string{
										someLabelKey:    someValueToPassThrough,
										"another/label": "label",
									},
								},
								Spec: corev1.PersistentVolumeClaimSpec{
									AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceStorage: cacheSize,
										},
									},
								},
							},
						},
					},
				})
			})

			it("deletes a cache if already exists and not requested", func() {
				image.Status.BuildCacheName = image.CacheName()
				image.Spec.Cache.Volume.Size = nil

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image.SourceResolver(),
						&corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      image.CacheName(),
								Namespace: image.Namespace,
							},
						},
						image,
						builder,
					},
					WantErr: false,
					WantDeletes: []clientgotesting.DeleteActionImpl{
						{
							ActionImpl: clientgotesting.ActionImpl{
								Namespace: "some-namespace",
								Resource: schema.GroupVersionResource{
									Resource: "persistentvolumeclaims",
								},
							},
							Name: image.CacheName(),
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionReadyUnknown(),
									},
								},
							},
						},
					},
				})
			})
		})

		when("reconciling builds", func() {
			it("does not schedule a build if the source resolver is not ready", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						unresolvedSourceResolver(image),
					},
					WantErr: false,
				})
			})

			it("does not schedule a build if the builder is not ready", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						notReadyBuilder(builder),
						resolvedSourceResolver(image),
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionUnknown,
											},
											{
												Type:    buildapi.ConditionBuilderReady,
												Status:  corev1.ConditionFalse,
												Reason:  buildapi.BuilderNotReady,
												Message: "Builder builder-name is not ready",
											},
										},
									},
								},
							},
						},
					},
				})
			})

			it("schedules a build if no build has been scheduled", func() {
				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						sourceResolver,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-1-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation: buildapi.BuildReasonConfig,
									buildapi.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {}
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url-resolved",
          "revision": "1234567-resolved"
        }
      }
    }
  }
]`),
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Cache:          &buildapi.BuildCacheConfig{},
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-1-00001"),
									},
									LatestBuildRef:             "image-name-build-1-00001", // GenerateNameReactor
									LatestBuildReason:          "CONFIG",
									LatestBuildImageGeneration: originalGeneration,
									BuildCounter:               1,
								},
							},
						},
					},
				})
			})

			it("schedules a build with a cluster builder", func() {
				image.Spec.Builder = corev1.ObjectReference{
					Kind: buildapi.ClusterBuilderKind,
					Name: clusterBuilderName,
				}

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						clusterBuilder,
						sourceResolver,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-1-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation: buildapi.BuildReasonConfig,
									buildapi.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {}
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url-resolved",
          "revision": "1234567-resolved"
        }
      }
    }
  }
]`),
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: clusterBuilder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Cache:          &buildapi.BuildCacheConfig{},
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-1-00001"),
									},
									LatestBuildRef:             "image-name-build-1-00001", // GenerateNameReactor
									LatestBuildReason:          "CONFIG",
									LatestBuildImageGeneration: originalGeneration,
									BuildCounter:               1,
								},
							},
						},
					},
				})
			})

			it("schedules a build with a builder", func() {
				image.Spec.Builder = corev1.ObjectReference{
					Kind: buildapi.BuilderKind,
					Name: builderName,
				}

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						builder,
						sourceResolver,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-1-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation: buildapi.BuildReasonConfig,
									buildapi.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {}
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url-resolved",
          "revision": "1234567-resolved"
        }
      }
    }
  }
]`),
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Cache:          &buildapi.BuildCacheConfig{},
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-1-00001"),
									},
									LatestBuildRef:             "image-name-build-1-00001", // GenerateNameReactor
									LatestBuildReason:          "CONFIG",
									LatestBuildImageGeneration: originalGeneration,
									BuildCounter:               1,
								},
							},
						},
					},
				})
			})

			it("schedules a build with a cluster builder", func() {
				image.Spec.Builder = corev1.ObjectReference{
					Kind: buildapi.ClusterBuilderKind,
					Name: clusterBuilderName,
				}

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						builder,
						clusterBuilder,
						sourceResolver,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-1-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation: buildapi.BuildReasonConfig,
									buildapi.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {}
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url-resolved",
          "revision": "1234567-resolved"
        }
      }
    }
  }
]`),
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: clusterBuilder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Cache:          &buildapi.BuildCacheConfig{},
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-1-00001"),
									},
									LatestBuildRef:             "image-name-build-1-00001", // GenerateNameReactor
									LatestBuildReason:          "CONFIG",
									LatestBuildImageGeneration: originalGeneration,
									BuildCounter:               1,
								},
							},
						},
					},
				})
			})

			it("schedules a build with a desired build cache", func() {
				cacheSize := resource.MustParse("2.5")
				image.Spec.Cache = &buildapi.ImageCacheConfig{
					Volume: &buildapi.ImagePersistentVolumeCache{
						Size: &cacheSize,
					},
				}
				image.Status.BuildCacheName = image.CacheName()

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						sourceResolver,
						image.BuildCache(),
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-1-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation: buildapi.BuildReasonConfig,
									buildapi.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {}
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url-resolved",
          "revision": "1234567-resolved"
        }
      }
    }
  }
]`),
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache: &buildapi.BuildCacheConfig{
									Volume: &buildapi.BuildPersistentVolumeCache{
										ClaimName: image.CacheName(),
									},
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-1-00001"),
									},
									LatestBuildRef:             "image-name-build-1-00001", // GenerateNameReactor
									LatestBuildReason:          "CONFIG",
									LatestBuildImageGeneration: originalGeneration,
									BuildCounter:               1,
									BuildCacheName:             image.CacheName(),
								},
							},
						},
					},
				})
			})

			it("schedules a build if the previous build does not match source", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-100001"

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1-00001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: "old-service-account",
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      "out-of-date-git-url",
										Revision: "out-of-date-git-revision",
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: image.Spec.Tag + "@sha256:just-built",
								Stack: corev1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
							},
						},
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-2-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "2",
									buildapi.ImageLabel:           imageName,
									someLabelKey:                  someValueToPassThrough,
									buildapi.ImageGenerationLabel: generation(image),
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation: strings.Join([]string{
										buildapi.BuildReasonCommit,
										buildapi.BuildReasonConfig,
									}, ","),
									buildapi.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "COMMIT",
    "old": "out-of-date-git-revision",
    "new": "1234567-resolved"
  },
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {
        "git": {
          "url": "out-of-date-git-url",
          "revision": "out-of-date-git-revision"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url-resolved",
          "revision": "1234567-resolved"
        }
      }
    }
  }
]`),
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache: &buildapi.BuildCacheConfig{},
								LastBuild: &buildapi.LastBuild{
									Image:   "some/image@sha256:just-built",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-2-00001"),
									},
									LatestBuildRef:             "image-name-build-2-00001", // GenerateNameReactor
									LatestBuildReason:          "COMMIT,CONFIG",
									LatestBuildImageGeneration: originalGeneration,
									LatestImage:                image.Spec.Tag + "@sha256:just-built",
									BuildCounter:               2,
								},
							},
						},
					},
				})
			})

			it("schedules a build when source resolver is updated", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-1-00001"

				sourceResolver := image.SourceResolver()
				sourceResolver.ResolvedSource(corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      image.Spec.Source.Git.URL,
						Revision: "new-commit",
						Type:     corev1alpha1.Branch,
					},
				})

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1-00001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation: buildapi.BuildReasonCommit,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      image.Spec.Source.Git.URL,
										Revision: image.Spec.Source.Git.Revision,
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: image.Spec.Tag + "@sha256:just-built",
								Stack: corev1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
							},
						},
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-2-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "2",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation:  buildapi.BuildReasonCommit,
									buildapi.BuildChangesAnnotation: `[{"reason":"COMMIT","old":"1234567","new":"new-commit"}]`,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache: &buildapi.BuildCacheConfig{},
								LastBuild: &buildapi.LastBuild{
									Image:   "some/image@sha256:just-built",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-2-00001"),
									},
									LatestBuildRef:             "image-name-build-2-00001", // GenerateNameReactor
									LatestBuildReason:          "COMMIT",
									LatestBuildImageGeneration: originalGeneration,
									LatestImage:                image.Spec.Tag + "@sha256:just-built",
									BuildCounter:               2,
								},
							},
						},
					},
				})
			})

			it("schedules a build when the builder buildpacks are updated", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-1-00001"
				const updatedBuilderImage = "some/builder@sha256:updated"

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						&buildapi.Builder{
							ObjectMeta: metav1.ObjectMeta{
								Name:      builderName,
								Namespace: namespace,
							},
							Status: buildapi.BuilderStatus{
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
								LatestImage: updatedBuilderImage,
								Stack: corev1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								BuilderMetadata: corev1alpha1.BuildpackMetadataList{
									{
										Id:      "io.buildpack",
										Version: "new-version",
									},
								},
							},
						},
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1-00001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: image.Spec.Tag + "@sha256:just-built",
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								Stack: corev1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								BuildMetadata: corev1alpha1.BuildpackMetadataList{
									{
										Id:      "io.buildpack",
										Version: "old-version",
									},
								},
							},
						},
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-2-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "2",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation: buildapi.BuildReasonBuildpack,
									buildapi.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "BUILDPACK",
    "old": [
      {
        "id": "io.buildpack",
        "version": "old-version"
      }
    ],
    "new": null
  }
]`),
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache: &buildapi.BuildCacheConfig{},
								LastBuild: &buildapi.LastBuild{
									Image:   "some/image@sha256:just-built",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-2-00001"),
									},
									LatestBuildRef:             "image-name-build-2-00001", // GenerateNameReactor
									LatestBuildReason:          "BUILDPACK",
									LatestBuildImageGeneration: originalGeneration,
									LatestImage:                image.Spec.Tag + "@sha256:just-built",
									BuildCounter:               2,
								},
							},
						},
					},
				})
			})

			it("schedules a build when the builder stack is updated", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-1-00001"
				const updatedBuilderImage = "some/builder@sha256:updated"

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						&buildapi.Builder{
							ObjectMeta: metav1.ObjectMeta{
								Name:      builderName,
								Namespace: namespace,
							},
							Status: buildapi.BuilderStatus{
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
								LatestImage: updatedBuilderImage,
								Stack: corev1alpha1.BuildStack{
									RunImage: "gcr.io/test-project/install/run@sha256:01ea3600f15a73f0ad445351c681eb0377738f5964cbcd2bab0cfec9ca891a08",
									ID:       "io.buildpacks.stacks.bionic",
								},
								BuilderMetadata: corev1alpha1.BuildpackMetadataList{
									{
										Id:      "io.buildpack",
										Version: "version",
									},
								},
							},
						},
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1-00001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: image.Spec.Tag + "@sha256:just-built",
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								Stack: corev1alpha1.BuildStack{
									RunImage: "gcr.io/test-project/install/run@sha256:42841631725942db48b7ba8b788b97374a2ada34c84ee02ca5e02ef3d4b0dfca",
									ID:       "io.buildpacks.stacks.bionic",
								},
								BuildMetadata: corev1alpha1.BuildpackMetadataList{
									{
										Id:      "io.buildpack",
										Version: "version",
									},
								},
							},
						},
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-2-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "2",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation: buildapi.BuildReasonStack,
									buildapi.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "STACK",
    "old": "sha256:42841631725942db48b7ba8b788b97374a2ada34c84ee02ca5e02ef3d4b0dfca",
    "new": "sha256:01ea3600f15a73f0ad445351c681eb0377738f5964cbcd2bab0cfec9ca891a08"
  }
]`),
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache: &buildapi.BuildCacheConfig{},
								LastBuild: &buildapi.LastBuild{
									Image:   "some/image@sha256:just-built",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-2-00001"),
									},
									LatestBuildRef:             "image-name-build-2-00001", // GenerateNameReactor
									LatestBuildImageGeneration: originalGeneration,
									LatestBuildReason:          buildapi.BuildReasonStack,
									LatestImage:                image.Spec.Tag + "@sha256:just-built",
									BuildCounter:               2,
								},
							},
						},
					},
				})
			})

			it("schedules a build with previous build's LastBuild if the last build failed", func() {
				image.Status.BuildCounter = 2
				image.Status.LatestBuildRef = "image-name-build200001"

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1-00001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "2",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: "old-service-account",
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      "out-of-date-git-url",
										Revision: "out-of-date-git-revision",
									},
								},
								LastBuild: &buildapi.LastBuild{
									Image:   image.Spec.Tag + "@sha256:from-build-before-this-build",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
							Status: buildapi.BuildStatus{
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionFalse,
										},
									},
								},
							},
						},
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-3-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "3",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuildReasonAnnotation: strings.Join([]string{
										buildapi.BuildReasonCommit,
										buildapi.BuildReasonConfig,
									}, ","),
									buildapi.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "COMMIT",
    "old": "out-of-date-git-revision",
    "new": "1234567-resolved"
  },
  {
    "reason": "CONFIG",
    "old": {
      "resources": {},
      "source": {
        "git": {
          "url": "out-of-date-git-url",
          "revision": "out-of-date-git-revision"
        }
      }
    },
    "new": {
      "resources": {},
      "source": {
        "git": {
          "url": "https://some.git/url-resolved",
          "revision": "1234567-resolved"
        }
      }
    }
  }
]`),
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache: &buildapi.BuildCacheConfig{},
								LastBuild: &buildapi.LastBuild{
									Image:   "some/image@sha256:from-build-before-this-build",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-3-00001"),
									},
									LatestBuildRef:             "image-name-build-3-00001", // GenerateNameReactor
									LatestBuildReason:          "COMMIT,CONFIG",
									LatestBuildImageGeneration: originalGeneration,
									BuildCounter:               3,
								},
							},
						},
					},
				})
			})

			it("does not schedule a build if the previous build is running", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-1"

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-100001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: "old-service-account",
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      "out-of-date-git-url",
										Revision: "out-of-date-git-revision",
									},
								},
							},
							Status: buildapi.BuildStatus{
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionUnknown,
										},
									},
								},
							},
						},
					},
					WantErr: false,
				})
			})

			it("does not schedule a build if the previous build spec matches the current desired spec", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-1"
				image.Status.LatestImage = "some/image@sha256:ad3f454c"
				image.Status.Conditions = conditionReady()
				image.Status.LatestStack = "io.buildpacks.stacks.bionic"

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      image.Status.LatestBuildRef,
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: image.Status.LatestImage,
								Stack: corev1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
										{
											Type:   buildapi.ConditionBuilderReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
							},
						},
					},
					WantErr: false,
				})
			})

			it("reports the last successful build on the image when the last build is successful", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-1"
				image.Status.LatestImage = "some/image@some-old-sha"
				image.Status.LatestStack = "io.buildpacks.stacks.bionic"

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: runtimeObjects(
						successfulBuilds(image, sourceResolver, 1),
						image,
						builder,
						sourceResolver,
					),
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionTrue,
											},
											{
												Type:   buildapi.ConditionBuilderReady,
												Status: corev1.ConditionTrue,
											},
										},
									},
									LatestBuildRef: "image-name-build-1",
									LatestImage:    "some/image@sha256:build-1",
									BuildCounter:   1,
									LatestStack:    "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
				})
			})

			it("reports unknown when last build was successful and source resolver is unknown", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-1"
				image.Status.LatestImage = "some/image@some-old-sha"
				image.Status.LatestStack = "io.buildpacks.stacks.bionic"

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: runtimeObjects(
						successfulBuilds(image, sourceResolver, 1),
						image,
						builder,
						unresolvedSourceResolver(image),
					),
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionUnknown,
											},
											{
												Type:   buildapi.ConditionBuilderReady,
												Status: corev1.ConditionTrue,
											},
										},
									},
									LatestBuildRef: "image-name-build-1",
									LatestImage:    "some/image@sha256:build-1",
									BuildCounter:   1,
									LatestStack:    "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
				})

			})

			it("reports unknown when last build was successful and builder is not ready", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-1"
				image.Status.LatestImage = "some/image@some-old-sha"
				image.Status.LatestStack = "io.buildpacks.stacks.bionic"

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: runtimeObjects(
						successfulBuilds(image, sourceResolver, 1),
						image,
						sourceResolver,
						notReadyBuilder(builder),
					),
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionUnknown,
											},
											{
												Type:    buildapi.ConditionBuilderReady,
												Status:  corev1.ConditionFalse,
												Reason:  buildapi.BuilderNotReady,
												Message: "Builder builder-name is not ready",
											},
										},
									},
									LatestBuildRef: "image-name-build-1",
									LatestImage:    "some/image@sha256:build-1",
									BuildCounter:   1,
									LatestStack:    "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
				})

			})

			when("reconciling old builds", func() {

				it("deletes a failed build if more than the limit", func() {
					image.Spec.FailedBuildHistoryLimit = limit(4)
					image.Status.LatestBuildRef = "image-name-build-5"
					image.Status.Conditions = conditionNotReady()
					image.Status.BuildCounter = 5
					sourceResolver := resolvedSourceResolver(image)

					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: runtimeObjects(
							failedBuilds(image, sourceResolver, 5),
							image,
							builder,
							sourceResolver,
						),
						WantErr: false,
						WantDeletes: []clientgotesting.DeleteActionImpl{
							{
								ActionImpl: clientgotesting.ActionImpl{
									Namespace: "some-namespace",
									Verb:      "",
									Resource: schema.GroupVersionResource{
										Resource: "builds",
									},
									Subresource: "",
								},
								Name: image.Name + "-build-1", // first-build
							},
						},
					})
				})

				it("deletes a successful build if more than the limit", func() {
					image.Spec.SuccessBuildHistoryLimit = limit(4)
					image.Status.LatestBuildRef = "image-name-build-5"
					image.Status.LatestImage = "some/image@sha256:build-5"
					image.Status.LatestStack = "io.buildpacks.stacks.bionic"
					image.Status.Conditions = conditionReady()
					image.Status.BuildCounter = 5
					sourceResolver := resolvedSourceResolver(image)

					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: runtimeObjects(
							successfulBuilds(image, sourceResolver, 5),
							image,
							builder,
							sourceResolver,
						),
						WantErr: false,
						WantDeletes: []clientgotesting.DeleteActionImpl{
							{
								ActionImpl: clientgotesting.ActionImpl{
									Namespace: "some-namespace",
									Verb:      "",
									Resource: schema.GroupVersionResource{
										Resource: "builds",
									},
									Subresource: "",
								},
								Name: image.Name + "-build-1", // first-build
							},
						},
					})
				})
			})
		})

		when("defaulting has not happened", func() {
			image.Spec.FailedBuildHistoryLimit = nil
			image.Spec.SuccessBuildHistoryLimit = nil

			it("sets the FailedBuildHistoryLimit and SuccessBuildHistoryLimit", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						clusterBuilder,
						unresolvedSourceResolver(image),
					},
					WantErr: false,
				})
			})
		})
	})
}

func generation(i *buildapi.Image) string {
	return strconv.Itoa(int(i.Generation))
}

func resolvedSourceResolver(image *buildapi.Image) *buildapi.SourceResolver {
	sr := image.SourceResolver()
	sr.ResolvedSource(corev1alpha1.ResolvedSourceConfig{
		Git: &corev1alpha1.ResolvedGitSource{
			URL:      image.Spec.Source.Git.URL + "-resolved",
			Revision: image.Spec.Source.Git.Revision + "-resolved",
			Type:     corev1alpha1.Branch,
		},
	})
	return sr
}

func unresolvedSourceResolver(image *buildapi.Image) *buildapi.SourceResolver {
	return image.SourceResolver()
}

func notReadyBuilder(builder *buildapi.Builder) runtime.Object {
	builder.Status.Conditions = corev1alpha1.Conditions{}
	return builder
}

func failedBuilds(image *buildapi.Image, sourceResolver *buildapi.SourceResolver, count int) []runtime.Object {
	return builds(image, sourceResolver, count, corev1alpha1.Condition{
		Type:   corev1alpha1.ConditionSucceeded,
		Status: corev1.ConditionFalse,
	})
}

func successfulBuilds(image *buildapi.Image, sourceResolver *buildapi.SourceResolver, count int) []runtime.Object {
	return builds(image, sourceResolver, count, corev1alpha1.Condition{
		Type:   corev1alpha1.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	})
}

func builds(image *buildapi.Image, sourceResolver *buildapi.SourceResolver, count int, condition corev1alpha1.Condition) []runtime.Object {
	var builds []runtime.Object
	const runImageRef = "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb"

	for i := 1; i <= count; i++ {
		builds = append(builds, &buildapi.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-build-%d", image.Name, i),
				Namespace: image.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*kmeta.NewControllerRef(image),
				},
				Labels: map[string]string{
					buildapi.BuildNumberLabel: fmt.Sprintf("%d", i),
					buildapi.ImageLabel:       image.Name,
				},
				CreationTimestamp: metav1.NewTime(time.Now().Add(time.Duration(i) * time.Minute)),
			},
			Spec: buildapi.BuildSpec{
				Tags: []string{image.Spec.Tag},
				Builder: corev1alpha1.BuildBuilderSpec{
					Image: "builder-image/foo@sha256:112312",
				},
				ServiceAccount: image.Spec.ServiceAccount,
				Source: corev1alpha1.SourceConfig{
					Git: &corev1alpha1.Git{
						URL:      sourceResolver.Status.Source.Git.URL,
						Revision: sourceResolver.Status.Source.Git.Revision,
					},
				},
			},
			Status: buildapi.BuildStatus{
				LatestImage: fmt.Sprintf("%s@sha256:build-%d", image.Spec.Tag, i),
				Stack: corev1alpha1.BuildStack{
					RunImage: runImageRef,
					ID:       "io.buildpacks.stacks.bionic",
				},
				Status: corev1alpha1.Status{
					Conditions: corev1alpha1.Conditions{
						condition,
					},
				},
			},
		})
	}

	return builds
}

func runtimeObjects(objects []runtime.Object, additional ...runtime.Object) []runtime.Object {
	return append(objects, additional...)
}

func limit(limit int64) *int64 {
	return &limit
}

func conditionReadyUnknown() corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:   corev1alpha1.ConditionReady,
			Status: corev1.ConditionUnknown,
		},
		{
			Type:   buildapi.ConditionBuilderReady,
			Status: corev1.ConditionTrue,
		},
	}
}

func conditionBuildExecuting(buildName string) corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:    corev1alpha1.ConditionReady,
			Status:  corev1.ConditionUnknown,
			Message: fmt.Sprintf("%s is executing", buildName),
		},
		{
			Type:   buildapi.ConditionBuilderReady,
			Status: corev1.ConditionTrue,
		},
	}
}

func conditionReady() corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:   corev1alpha1.ConditionReady,
			Status: corev1.ConditionTrue,
		},
		{
			Type:   buildapi.ConditionBuilderReady,
			Status: corev1.ConditionTrue,
		},
	}
}

func conditionNotReady() corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:   corev1alpha1.ConditionReady,
			Status: corev1.ConditionFalse,
		},
		{
			Type:   buildapi.ConditionBuilderReady,
			Status: corev1.ConditionTrue,
		},
	}
}
