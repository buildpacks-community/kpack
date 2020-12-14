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

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
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

	image := &v1alpha1.Image{
		ObjectMeta: metav1.ObjectMeta{
			Name:       imageName,
			Namespace:  namespace,
			Generation: originalGeneration,
			Labels: map[string]string{
				someLabelKey: someValueToPassThrough,
			},
		},
		Spec: v1alpha1.ImageSpec{
			Tag: "some/image",
			Builder: corev1.ObjectReference{
				Kind: "Builder",
				Name: builderName,
			},
			ServiceAccount: serviceAccount,
			Source: v1alpha1.SourceConfig{
				Git: &v1alpha1.Git{
					URL:      "https://some.git/url",
					Revision: "1234567",
				},
			},
			FailedBuildHistoryLimit:  limit(10),
			SuccessBuildHistoryLimit: limit(10),
			ImageTaggingStrategy:     v1alpha1.None,
			Build:                    &v1alpha1.ImageBuild{},
		},
		Status: v1alpha1.ImageStatus{
			Status: corev1alpha1.Status{
				ObservedGeneration: originalGeneration,
				Conditions:         conditionReadyUnknown(),
			},
		},
	}

	builder := &v1alpha1.Builder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      builderName,
			Namespace: namespace,
		},
		Status: v1alpha1.BuilderStatus{
			LatestImage: "some/builder@sha256:acf123",
			BuilderMetadata: v1alpha1.BuildpackMetadataList{
				{
					Id:      "buildpack.version",
					Version: "version",
				},
			},
			Stack: v1alpha1.BuildStack{
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

	clusterBuilder := &v1alpha1.ClusterBuilder{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterBuilderName,
		},
		Status: v1alpha1.BuilderStatus{
			LatestImage: "some/clusterbuilder@sha256:acf123",
			BuilderMetadata: v1alpha1.BuildpackMetadataList{
				{
					Id:      "buildpack.version",
					Version: "version",
				},
			},
			Stack: v1alpha1.BuildStack{
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
						Object: &v1alpha1.Image{
							ObjectMeta: image.ObjectMeta,
							Spec:       image.Spec,
							Status: v1alpha1.ImageStatus{
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
						Object: &v1alpha1.Image{
							ObjectMeta: image.ObjectMeta,
							Spec:       image.Spec,
							Status: v1alpha1.ImageStatus{
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
						&v1alpha1.SourceResolver{
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
							Spec: v1alpha1.SourceResolverSpec{
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
						&v1alpha1.SourceResolver{
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
							Spec: v1alpha1.SourceResolverSpec{
								ServiceAccount: "old-account",
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
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
							Object: &v1alpha1.SourceResolver{
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
								Spec: v1alpha1.SourceResolverSpec{
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
							Object: &v1alpha1.SourceResolver{
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
								Spec: v1alpha1.SourceResolverSpec{
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

			it("creates a cache if requested", func() {
				image.Spec.CacheSize = &cacheSize

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
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
				image.Spec.CacheSize = &cacheSize
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
				image.Spec.CacheSize = &newCacheSize

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
				image.Spec.CacheSize = &cacheSize
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
				image.Spec.CacheSize = nil

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
							Name: image.CacheName(),
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionUnknown,
											},
											{
												Type:    v1alpha1.ConditionBuilderReady,
												Status:  corev1.ConditionFalse,
												Reason:  v1alpha1.BuilderNotReady,
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-1-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel:     "1",
									v1alpha1.ImageLabel:           imageName,
									v1alpha1.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonConfig,
									v1alpha1.BuildChangesAnnotation: testhelpers.CompactJSON(`
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
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
					Kind: v1alpha1.ClusterBuilderKind,
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-1-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel:     "1",
									v1alpha1.ImageLabel:           imageName,
									v1alpha1.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonConfig,
									v1alpha1.BuildChangesAnnotation: testhelpers.CompactJSON(`
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
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: clusterBuilder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
					Kind: v1alpha1.BuilderKind,
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-1-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel:     "1",
									v1alpha1.ImageLabel:           imageName,
									v1alpha1.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonConfig,
									v1alpha1.BuildChangesAnnotation: testhelpers.CompactJSON(`
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
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
					Kind: v1alpha1.ClusterBuilderKind,
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-1-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel:     "1",
									v1alpha1.ImageLabel:           imageName,
									v1alpha1.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonConfig,
									v1alpha1.BuildChangesAnnotation: testhelpers.CompactJSON(`
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
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: clusterBuilder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
				image.Spec.CacheSize = &cacheSize
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-1-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel:     "1",
									v1alpha1.ImageLabel:           imageName,
									v1alpha1.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonConfig,
									v1alpha1.BuildChangesAnnotation: testhelpers.CompactJSON(`
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
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								CacheName: image.CacheName(),
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1-00001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel: "1",
									v1alpha1.ImageLabel:       imageName,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: "old-service-account",
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      "out-of-date-git-url",
										Revision: "out-of-date-git-revision",
									},
								},
							},
							Status: v1alpha1.BuildStatus{
								LatestImage: image.Spec.Tag + "@sha256:just-built",
								Stack: v1alpha1.BuildStack{
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-2-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel:     "2",
									v1alpha1.ImageLabel:           imageName,
									someLabelKey:                  someValueToPassThrough,
									v1alpha1.ImageGenerationLabel: generation(image),
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: strings.Join([]string{
										v1alpha1.BuildReasonCommit,
										v1alpha1.BuildReasonConfig,
									}, ","),
									v1alpha1.BuildChangesAnnotation: testhelpers.CompactJSON(`
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
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								LastBuild: &v1alpha1.LastBuild{
									Image:   "some/image@sha256:just-built",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
				sourceResolver.ResolvedSource(v1alpha1.ResolvedSourceConfig{
					Git: &v1alpha1.ResolvedGitSource{
						URL:      image.Spec.Source.Git.URL,
						Revision: "new-commit",
						Type:     v1alpha1.Branch,
					},
				})

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						builder,
						sourceResolver,
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1-00001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel: "1",
									v1alpha1.ImageLabel:       imageName,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonCommit,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      image.Spec.Source.Git.URL,
										Revision: image.Spec.Source.Git.Revision,
									},
								},
							},
							Status: v1alpha1.BuildStatus{
								LatestImage: image.Spec.Tag + "@sha256:just-built",
								Stack: v1alpha1.BuildStack{
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-2-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel:     "2",
									v1alpha1.ImageLabel:           imageName,
									v1alpha1.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation:  v1alpha1.BuildReasonCommit,
									v1alpha1.BuildChangesAnnotation: `[{"reason":"COMMIT","old":"1234567","new":"new-commit"}]`,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								LastBuild: &v1alpha1.LastBuild{
									Image:   "some/image@sha256:just-built",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
						&v1alpha1.Builder{
							ObjectMeta: metav1.ObjectMeta{
								Name:      builderName,
								Namespace: namespace,
							},
							Status: v1alpha1.BuilderStatus{
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
								LatestImage: updatedBuilderImage,
								Stack: v1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								BuilderMetadata: v1alpha1.BuildpackMetadataList{
									{
										Id:      "io.buildpack",
										Version: "new-version",
									},
								},
							},
						},
						sourceResolver,
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1-00001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel: "1",
									v1alpha1.ImageLabel:       imageName,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
							Status: v1alpha1.BuildStatus{
								LatestImage: image.Spec.Tag + "@sha256:just-built",
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								Stack: v1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								BuildMetadata: v1alpha1.BuildpackMetadataList{
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-2-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel:     "2",
									v1alpha1.ImageLabel:           imageName,
									v1alpha1.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonBuildpack,
									v1alpha1.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "BUILDPACK",
    "old": [
      {
        "id": "io.buildpack",
        "version": "old-version"
      }
    ],
    "new": [
      {
        "id": "io.buildpack",
        "version": "new-version"
      }
    ]
  }
]`),
								},
							},
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								LastBuild: &v1alpha1.LastBuild{
									Image:   "some/image@sha256:just-built",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
						&v1alpha1.Builder{
							ObjectMeta: metav1.ObjectMeta{
								Name:      builderName,
								Namespace: namespace,
							},
							Status: v1alpha1.BuilderStatus{
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
									},
								},
								LatestImage: updatedBuilderImage,
								Stack: v1alpha1.BuildStack{
									RunImage: "gcr.io/test-project/install/run@sha256:01ea3600f15a73f0ad445351c681eb0377738f5964cbcd2bab0cfec9ca891a08",
									ID:       "io.buildpacks.stacks.bionic",
								},
								BuilderMetadata: v1alpha1.BuildpackMetadataList{
									{
										Id:      "io.buildpack",
										Version: "version",
									},
								},
							},
						},
						sourceResolver,
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1-00001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel: "1",
									v1alpha1.ImageLabel:       imageName,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
							Status: v1alpha1.BuildStatus{
								LatestImage: image.Spec.Tag + "@sha256:just-built",
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								Stack: v1alpha1.BuildStack{
									RunImage: "gcr.io/test-project/install/run@sha256:42841631725942db48b7ba8b788b97374a2ada34c84ee02ca5e02ef3d4b0dfca",
									ID:       "io.buildpacks.stacks.bionic",
								},
								BuildMetadata: v1alpha1.BuildpackMetadataList{
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-2-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel:     "2",
									v1alpha1.ImageLabel:           imageName,
									v1alpha1.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonStack,
									v1alpha1.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "STACK",
    "old": "sha256:42841631725942db48b7ba8b788b97374a2ada34c84ee02ca5e02ef3d4b0dfca",
    "new": "sha256:01ea3600f15a73f0ad445351c681eb0377738f5964cbcd2bab0cfec9ca891a08"
  }
]`),
								},
							},
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								LastBuild: &v1alpha1.LastBuild{
									Image:   "some/image@sha256:just-built",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-2-00001"),
									},
									LatestBuildRef:             "image-name-build-2-00001", // GenerateNameReactor
									LatestBuildImageGeneration: originalGeneration,
									LatestBuildReason:          v1alpha1.BuildReasonStack,
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1-00001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel: "2",
									v1alpha1.ImageLabel:       imageName,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: "old-service-account",
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      "out-of-date-git-url",
										Revision: "out-of-date-git-revision",
									},
								},
								LastBuild: &v1alpha1.LastBuild{
									Image:   image.Spec.Tag + "@sha256:from-build-before-this-build",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
							Status: v1alpha1.BuildStatus{
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								GenerateName: imageName + "-build-3-",
								Namespace:    namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel:     "3",
									v1alpha1.ImageLabel:           imageName,
									v1alpha1.ImageGenerationLabel: generation(image),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: strings.Join([]string{
										v1alpha1.BuildReasonCommit,
										v1alpha1.BuildReasonConfig,
									}, ","),
									v1alpha1.BuildChangesAnnotation: testhelpers.CompactJSON(`
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
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								LastBuild: &v1alpha1.LastBuild{
									Image:   "some/image@sha256:from-build-before-this-build",
									StackId: "io.buildpacks.stacks.bionic",
								},
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-100001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel: "1",
									v1alpha1.ImageLabel:       imageName,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: "old-service-account",
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      "out-of-date-git-url",
										Revision: "out-of-date-git-revision",
									},
								},
							},
							Status: v1alpha1.BuildStatus{
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
						&v1alpha1.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      image.Status.LatestBuildRef,
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(image),
								},
								Labels: map[string]string{
									v1alpha1.BuildNumberLabel: "1",
									v1alpha1.ImageLabel:       imageName,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Tags: []string{image.Spec.Tag},
								Builder: v1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.SourceConfig{
									Git: &v1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
							Status: v1alpha1.BuildStatus{
								LatestImage: image.Status.LatestImage,
								Stack: v1alpha1.BuildStack{
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
											Type:   v1alpha1.ConditionBuilderReady,
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
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionTrue,
											},
											{
												Type:   v1alpha1.ConditionBuilderReady,
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
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionUnknown,
											},
											{
												Type:   v1alpha1.ConditionBuilderReady,
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
							Object: &v1alpha1.Image{
								ObjectMeta: image.ObjectMeta,
								Spec:       image.Spec,
								Status: v1alpha1.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionUnknown,
											},
											{
												Type:    v1alpha1.ConditionBuilderReady,
												Status:  corev1.ConditionFalse,
												Reason:  v1alpha1.BuilderNotReady,
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
									Namespace:   "blah",
									Verb:        "",
									Resource:    schema.GroupVersionResource{},
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
									Namespace:   "blah",
									Verb:        "",
									Resource:    schema.GroupVersionResource{},
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

func generation(i *v1alpha1.Image) string {
	return strconv.Itoa(int(i.Generation))
}

func resolvedSourceResolver(image *v1alpha1.Image) *v1alpha1.SourceResolver {
	sr := image.SourceResolver()
	sr.ResolvedSource(v1alpha1.ResolvedSourceConfig{
		Git: &v1alpha1.ResolvedGitSource{
			URL:      image.Spec.Source.Git.URL + "-resolved",
			Revision: image.Spec.Source.Git.Revision + "-resolved",
			Type:     v1alpha1.Branch,
		},
	})
	return sr
}

func unresolvedSourceResolver(image *v1alpha1.Image) *v1alpha1.SourceResolver {
	return image.SourceResolver()
}

func notReadyBuilder(builder *v1alpha1.Builder) runtime.Object {
	builder.Status.Conditions = corev1alpha1.Conditions{}
	return builder
}

func failedBuilds(image *v1alpha1.Image, sourceResolver *v1alpha1.SourceResolver, count int) []runtime.Object {
	return builds(image, sourceResolver, count, corev1alpha1.Condition{
		Type:   corev1alpha1.ConditionSucceeded,
		Status: corev1.ConditionFalse,
	})
}

func successfulBuilds(image *v1alpha1.Image, sourceResolver *v1alpha1.SourceResolver, count int) []runtime.Object {
	return builds(image, sourceResolver, count, corev1alpha1.Condition{
		Type:   corev1alpha1.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	})
}

func builds(image *v1alpha1.Image, sourceResolver *v1alpha1.SourceResolver, count int, condition corev1alpha1.Condition) []runtime.Object {
	var builds []runtime.Object
	const runImageRef = "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb"

	for i := 1; i <= count; i++ {
		builds = append(builds, &v1alpha1.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-build-%d", image.Name, i),
				Namespace: image.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					*kmeta.NewControllerRef(image),
				},
				Labels: map[string]string{
					v1alpha1.BuildNumberLabel: fmt.Sprintf("%d", i),
					v1alpha1.ImageLabel:       image.Name,
				},
				CreationTimestamp: metav1.NewTime(time.Now().Add(time.Duration(i) * time.Minute)),
			},
			Spec: v1alpha1.BuildSpec{
				Tags: []string{image.Spec.Tag},
				Builder: v1alpha1.BuildBuilderSpec{
					Image: "builder-image/foo@sha256:112312",
				},
				ServiceAccount: image.Spec.ServiceAccount,
				Source: v1alpha1.SourceConfig{
					Git: &v1alpha1.Git{
						URL:      sourceResolver.Status.Source.Git.URL,
						Revision: sourceResolver.Status.Source.Git.Revision,
					},
				},
			},
			Status: v1alpha1.BuildStatus{
				LatestImage: fmt.Sprintf("%s@sha256:build-%d", image.Spec.Tag, i),
				Stack: v1alpha1.BuildStack{
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
			Type:   v1alpha1.ConditionBuilderReady,
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
			Type:   v1alpha1.ConditionBuilderReady,
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
			Type:   v1alpha1.ConditionBuilderReady,
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
			Type:   v1alpha1.ConditionBuilderReady,
			Status: corev1.ConditionTrue,
		},
	}
}
