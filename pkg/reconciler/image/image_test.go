package image_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
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
	"github.com/pivotal/kpack/pkg/reconciler"
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
	fakeTracker := &testhelpers.FakeTracker{}

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

	imageWithBuilder := &buildapi.Image{
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
			ServiceAccountName: serviceAccount,
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

	imageWithClusterBuilder := &buildapi.Image{
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
				Kind: "ClusterBuilder",
				Name: clusterBuilderName,
			},
			ServiceAccountName: serviceAccount,
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
		TypeMeta: metav1.TypeMeta{
			Kind:       buildapi.BuilderKind,
			APIVersion: "kpack.io/v1alpha2",
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
			Lifecycle: buildapi.ResolvedClusterLifecycle{
				Version: "some-version",
			},
			Status: corev1alpha1.Status{
				Conditions: corev1alpha1.Conditions{
					{
						Type:   corev1alpha1.ConditionReady,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   buildapi.ConditionUpToDate,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	builderRunImage := buildapi.BuildSpecImage{
		Image: builder.Status.Stack.RunImage,
	}

	clusterBuilder := &buildapi.ClusterBuilder{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterBuilderName,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       buildapi.ClusterBuilderKind,
			APIVersion: "kpack.io/v1alpha2",
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
					{
						Type:   buildapi.ConditionUpToDate,
						Status: corev1.ConditionTrue,
					},
				},
			},
		},
	}

	when("Reconcile", func() {
		it("updates observed generation after processing an update", func() {
			const updatedGeneration int64 = 2
			imageWithBuilder.ObjectMeta.Generation = updatedGeneration

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					imageWithBuilder,
					builder,
					clusterBuilder,
					unresolvedSourceResolver(imageWithBuilder),
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Image{
							ObjectMeta: imageWithBuilder.ObjectMeta,
							Spec:       imageWithBuilder.Spec,
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
					imageWithBuilder,
					builder,
					clusterBuilder,
					unresolvedSourceResolver(imageWithBuilder),
				},
				WantErr: false,
			})
		})

		it("tracks builder for image", func() {
			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					imageWithBuilder,
					builder,
					clusterBuilder,
					unresolvedSourceResolver(imageWithBuilder),
				},
				WantErr: false,
			})

			require.True(t, fakeTracker.IsTracking(
				reconciler.KeyForObject(builder).WithNamespace(namespace),
				imageWithBuilder.NamespacedName()))
		})

		it("tracks clusterbuilder for image", func() {
			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					imageWithClusterBuilder,
					builder,
					clusterBuilder,
					unresolvedSourceResolver(imageWithClusterBuilder),
				},
				WantErr: false,
			})

			require.True(t, fakeTracker.IsTracking(
				reconciler.KeyForObject(clusterBuilder),
				imageWithClusterBuilder.NamespacedName()))
		})

		it("sets condition not ready for non-existent builder", func() {
			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					imageWithBuilder,
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &buildapi.Image{
							ObjectMeta: imageWithBuilder.ObjectMeta,
							Spec:       imageWithBuilder.Spec,
							Status: buildapi.ImageStatus{
								Status: corev1alpha1.Status{
									ObservedGeneration: originalGeneration,
									Conditions: corev1alpha1.Conditions{
										{
											Type:    corev1alpha1.ConditionReady,
											Status:  corev1.ConditionFalse,
											Reason:  "BuilderNotFound",
											Message: "Error: Unable to find builder 'builder-name' in namespace ''.",
										},
									},
								},
							},
						},
					},
				},
			})

			// still track resource
			require.True(t, fakeTracker.IsTracking(
				reconciler.KeyForObject(builder).WithNamespace(namespace),
				imageWithBuilder.NamespacedName()))
		})

		when("reconciling source resolvers", func() {
			it("creates a source resolver if not created", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.SourceResolver{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageWithBuilder.SourceResolverName(),
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									someLabelKey: someValueToPassThrough,
								},
							},
							Spec: buildapi.SourceResolverSpec{
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source:             imageWithBuilder.Spec.Source,
							},
						},
					},
				})
			})

			it("does not create a source resolver if already created", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						imageWithBuilder.SourceResolver(),
					},
					WantErr: false,
				})
			})

			it("updates source resolver if configuration changed", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						&buildapi.SourceResolver{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageWithBuilder.SourceResolverName(),
								Namespace: namespace,
								Labels: map[string]string{
									someLabelKey: someValueToPassThrough,
								},
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
							},
							Spec: buildapi.SourceResolverSpec{
								ServiceAccountName: "old-account",
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
									Name:      imageWithBuilder.SourceResolverName(),
									Namespace: namespace,
									Labels: map[string]string{
										someLabelKey: someValueToPassThrough,
									},
									OwnerReferences: []metav1.OwnerReference{
										*kmeta.NewControllerRef(imageWithBuilder),
									},
								},
								Spec: buildapi.SourceResolverSpec{
									ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
									Source:             imageWithBuilder.Spec.Source,
								},
							},
						},
					},
				})
			})

			it("updates source resolver if labels change", func() {
				sourceResolver := imageWithBuilder.SourceResolver()

				extraLabelImage := imageWithBuilder.DeepCopy()
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
									Name:      imageWithBuilder.SourceResolverName(),
									Namespace: namespace,
									OwnerReferences: []metav1.OwnerReference{
										*kmeta.NewControllerRef(imageWithBuilder),
									},
									Labels: map[string]string{
										someLabelKey:    someValueToPassThrough,
										"another/label": "label",
									},
								},
								Spec: buildapi.SourceResolverSpec{
									ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
									Source:             imageWithBuilder.Spec.Source,
								},
							},
						},
					},
				})
			})
		})

		when("reconciling build caches", func() {
			cacheSize := resource.MustParse("1.5")
			imageWithBuilder.Spec.Cache = &buildapi.ImageCacheConfig{
				Volume: &buildapi.ImagePersistentVolumeCache{
					Size: &cacheSize,
				},
			}

			it("creates a cache if requested", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						imageWithBuilder.SourceResolver(),
						builder,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageWithBuilder.CacheName(),
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									someLabelKey: someValueToPassThrough,
								},
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								Resources: corev1.VolumeResourceRequirements{
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									BuildCacheName: imageWithBuilder.CacheName(),
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
				imageWithBuilder.Spec.Cache.Volume.Size = &cacheSize
				imageWithBuilder.Status.BuildCacheName = imageWithBuilder.CacheName()
				storageClassName := "some-name"

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						imageWithBuilder.SourceResolver(),
						&corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageWithBuilder.CacheName(),
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									someLabelKey: someValueToPassThrough,
								},
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: cacheSize,
									},
								},
								StorageClassName: &storageClassName,
							},
						},
						builder,
					},
					WantErr: false,
				})
			})

			it("updates build cache if size changes", func() {
				imageCacheName := imageWithBuilder.CacheName()

				imageWithBuilder.Status.BuildCacheName = imageCacheName
				newCacheSize := resource.MustParse("2.5")
				imageWithBuilder.Spec.Cache.Volume.Size = &newCacheSize

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						imageWithBuilder.SourceResolver(),
						builder,
						&corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageCacheName,
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								Resources: corev1.VolumeResourceRequirements{
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
										*kmeta.NewControllerRef(imageWithBuilder),
									},
								},
								Spec: corev1.PersistentVolumeClaimSpec{
									AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
									Resources: corev1.VolumeResourceRequirements{
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
				imageCacheName := imageWithBuilder.CacheName()
				imageWithBuilder.Spec.Cache.Volume.Size = &cacheSize
				imageWithBuilder.Status.BuildCacheName = imageCacheName
				cache := imageWithBuilder.BuildCache()

				extraLabelImage := imageWithBuilder.DeepCopy()
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
										*kmeta.NewControllerRef(imageWithBuilder),
									},
									Namespace: namespace,
									Labels: map[string]string{
										someLabelKey:    someValueToPassThrough,
										"another/label": "label",
									},
								},
								Spec: corev1.PersistentVolumeClaimSpec{
									AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
									Resources: corev1.VolumeResourceRequirements{
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
				imageWithBuilder.Status.BuildCacheName = imageWithBuilder.CacheName()
				imageWithBuilder.Spec.Cache.Volume.Size = nil

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder.SourceResolver(),
						&corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageWithBuilder.CacheName(),
								Namespace: imageWithBuilder.Namespace,
							},
						},
						imageWithBuilder,
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
							Name: imageWithBuilder.CacheName(),
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
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

			it("uses the storageClassName if provided", func() {
				imageWithBuilder.Spec.Cache.Volume.StorageClassName = "some-storage-class"
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						imageWithBuilder.SourceResolver(),
						builder,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&corev1.PersistentVolumeClaim{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageWithBuilder.CacheName(),
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									someLabelKey: someValueToPassThrough,
								},
							},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								Resources: corev1.VolumeResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceStorage: cacheSize,
									},
								},
								StorageClassName: &imageWithBuilder.Spec.Cache.Volume.StorageClassName,
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									BuildCacheName: imageWithBuilder.CacheName(),
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
						imageWithBuilder,
						builder,
						unresolvedSourceResolver(imageWithBuilder),
					},
					WantErr: false,
					//no builds are created
					WantCreates: nil,
				})

				assert.Equal(t, "Error: SourceResolver 'image-name-source' is not ready", imageWithBuilder.Status.GetCondition(corev1alpha1.ConditionReady).Message)
			})

			it("does not schedule a build if the builder is not ready", func() {

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builderWithCondition(
							builder,
							corev1alpha1.Condition{
								Type:    corev1alpha1.ConditionReady,
								Status:  corev1.ConditionFalse,
								Message: "something went wrong",
							},
							corev1alpha1.Condition{
								Type:   buildapi.ConditionUpToDate,
								Status: corev1.ConditionFalse,
							},
						),
						resolvedSourceResolver(imageWithBuilder),
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:    corev1alpha1.ConditionReady,
												Status:  corev1.ConditionFalse,
												Reason:  buildapi.BuilderNotReady,
												Message: "Error: Builder 'builder-name' is not ready in namespace 'some-namespace'; Message: something went wrong",
											},
											{
												Type:    buildapi.ConditionBuilderReady,
												Status:  corev1.ConditionFalse,
												Reason:  buildapi.BuilderNotReady,
												Message: "Error: Builder 'builder-name' is not ready in namespace 'some-namespace'; Message: something went wrong",
											},
											{
												Type:    buildapi.ConditionBuilderUpToDate,
												Status:  corev1.ConditionFalse,
												Reason:  buildapi.BuilderNotUpToDate,
												Message: "Builder is not up to date. The latest stack and buildpacks may not be in use.",
											},
										},
									},
								},
							},
						},
					},
				})
			})

			it("includes builder UpToDate condition in status", func() {
				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builderWithCondition(
							builder,
							corev1alpha1.Condition{
								Type:   corev1alpha1.ConditionReady,
								Status: corev1.ConditionTrue,
							},
							corev1alpha1.Condition{
								Type:    buildapi.ConditionUpToDate,
								Status:  corev1.ConditionFalse,
								Message: "Builder failed to reconcile",
								Reason:  buildapi.ReconcileFailedReason,
							},
						),
						sourceResolver,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageName + "-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: builderName,
									buildapi.BuilderKindAnnotation: buildapi.BuilderKind,
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Cache:              &buildapi.BuildCacheConfig{},
								RunImage:           builderRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									LatestBuildRef:             "image-name-build-1",
									LatestBuildImageGeneration: 1,
									BuildCounter:               1,
									LatestBuildReason:          "CONFIG",
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:    corev1alpha1.ConditionReady,
												Status:  corev1.ConditionUnknown,
												Reason:  image.BuildRunningReason,
												Message: "Build 'image-name-build-1' is executing",
											},
											{
												Type:   buildapi.ConditionBuilderReady,
												Status: corev1.ConditionTrue,
												Reason: buildapi.BuilderReady,
											},
											{
												Type:    buildapi.ConditionBuilderUpToDate,
												Status:  corev1.ConditionFalse,
												Reason:  buildapi.BuilderNotUpToDate,
												Message: "Builder is not up to date. The latest stack and buildpacks may not be in use.",
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
				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						sourceResolver,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageName + "-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: builderName,
									buildapi.BuilderKindAnnotation: buildapi.BuilderKind,
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Cache:              &buildapi.BuildCacheConfig{},
								RunImage:           builderRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-1"),
									},
									LatestBuildRef:             "image-name-build-1",
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
				imageWithBuilder.Spec.Builder = corev1.ObjectReference{
					Kind: buildapi.ClusterBuilderKind,
					Name: clusterBuilderName,
				}

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						clusterBuilder,
						sourceResolver,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageName + "-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: clusterBuilderName,
									buildapi.BuilderKindAnnotation: buildapi.ClusterBuilderKind,
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: clusterBuilder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Cache:              &buildapi.BuildCacheConfig{},
								RunImage:           builderRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-1"),
									},
									LatestBuildRef:             "image-name-build-1",
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
				imageWithBuilder.Spec.Builder = corev1.ObjectReference{
					Kind: buildapi.BuilderKind,
					Name: builderName,
				}

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						builder,
						sourceResolver,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageName + "-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: builderName,
									buildapi.BuilderKindAnnotation: buildapi.BuilderKind,
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Cache:              &buildapi.BuildCacheConfig{},
								RunImage:           builderRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-1"),
									},
									LatestBuildRef:             "image-name-build-1",
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
				imageWithBuilder.Spec.Builder = corev1.ObjectReference{
					Kind: buildapi.ClusterBuilderKind,
					Name: clusterBuilderName,
				}

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						builder,
						clusterBuilder,
						sourceResolver,
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageName + "-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: clusterBuilderName,
									buildapi.BuilderKindAnnotation: buildapi.ClusterBuilderKind,
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: clusterBuilder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Cache:              &buildapi.BuildCacheConfig{},
								RunImage:           builderRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-1"),
									},
									LatestBuildRef:             "image-name-build-1",
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
				imageWithBuilder.Spec.Cache = &buildapi.ImageCacheConfig{
					Volume: &buildapi.ImagePersistentVolumeCache{
						Size: &cacheSize,
					},
				}
				imageWithBuilder.Status.BuildCacheName = imageWithBuilder.CacheName()

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						sourceResolver,
						imageWithBuilder.BuildCache(),
					},
					WantErr: false,
					WantCreates: []runtime.Object{
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageName + "-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "1",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: builderName,
									buildapi.BuilderKindAnnotation: buildapi.BuilderKind,
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache: &buildapi.BuildCacheConfig{
									Volume: &buildapi.BuildPersistentVolumeCache{
										ClaimName: imageWithBuilder.CacheName(),
									},
								},
								RunImage: builderRunImage,
							},
						},
					},
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-1"),
									},
									LatestBuildRef:             "image-name-build-1",
									LatestBuildReason:          "CONFIG",
									LatestBuildImageGeneration: originalGeneration,
									BuildCounter:               1,
									BuildCacheName:             imageWithBuilder.CacheName(),
								},
							},
						},
					},
				})
			})

			it("schedules a build if the previous build does not match source", func() {
				imageWithBuilder.Status.BuildCounter = 1
				imageWithBuilder.Status.LatestBuildRef = "image-name-build-100001"

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: "old-service-account",
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      "out-of-date-git-url",
										Revision: "out-of-date-git-revision",
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: imageWithBuilder.Spec.Tag + "@sha256:just-built",
								Stack: corev1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								LifecycleVersion: "some-version",
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
								Name:      imageName + "-build-2",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "2",
									buildapi.ImageLabel:           imageName,
									someLabelKey:                  someValueToPassThrough,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: builderName,
									buildapi.BuilderKindAnnotation: buildapi.BuilderKind,
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache:    &buildapi.BuildCacheConfig{},
								RunImage: builderRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-2"),
									},
									LatestBuildRef:             "image-name-build-2",
									LatestBuildReason:          "COMMIT,CONFIG",
									LatestBuildImageGeneration: originalGeneration,
									LatestImage:                imageWithBuilder.Spec.Tag + "@sha256:just-built",
									BuildCounter:               2,
								},
							},
						},
					},
				})
			})

			it("schedules a build when source resolver is updated", func() {
				imageWithBuilder.Status.BuildCounter = 1
				imageWithBuilder.Status.LatestBuildRef = "image-name-build-1"

				sourceResolver := imageWithBuilder.SourceResolver()
				sourceResolver.ResolvedSource(corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      imageWithBuilder.Spec.Source.Git.URL,
						Revision: "new-commit",
						Type:     corev1alpha1.Branch,
					},
				})

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      imageWithBuilder.Spec.Source.Git.URL,
										Revision: imageWithBuilder.Spec.Source.Git.Revision,
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: imageWithBuilder.Spec.Tag + "@sha256:just-built",
								Stack: corev1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								LifecycleVersion: "some-version",
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
								Name:      imageName + "-build-2",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "2",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation:  builderName,
									buildapi.BuilderKindAnnotation:  buildapi.BuilderKind,
									buildapi.BuildReasonAnnotation:  buildapi.BuildReasonCommit,
									buildapi.BuildChangesAnnotation: `[{"reason":"COMMIT","old":"1234567","new":"new-commit"}]`,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache:    &buildapi.BuildCacheConfig{},
								RunImage: builderRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-2"),
									},
									LatestBuildRef:             "image-name-build-2",
									LatestBuildReason:          "COMMIT",
									LatestBuildImageGeneration: originalGeneration,
									LatestImage:                imageWithBuilder.Spec.Tag + "@sha256:just-built",
									BuildCounter:               2,
								},
							},
						},
					},
				})
			})

			it("schedules a build when the builder buildpacks are updated", func() {
				imageWithBuilder.Status.BuildCounter = 1
				imageWithBuilder.Status.LatestBuildRef = "image-name-build-1"
				const updatedBuilderImage = "some/builder@sha256:updated"

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						&buildapi.Builder{
							ObjectMeta: metav1.ObjectMeta{
								Name:      builderName,
								Namespace: namespace,
							},
							TypeMeta: metav1.TypeMeta{
								Kind: buildapi.BuilderKind,
							},
							Status: buildapi.BuilderStatus{
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
										{
											Type:   buildapi.ConditionUpToDate,
											Status: corev1.ConditionTrue,
										},
									},
								},
								LatestImage: updatedBuilderImage,
								Stack: corev1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								Lifecycle: buildapi.ResolvedClusterLifecycle{
									Version: "some-version",
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
								Name:      "image-name-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: imageWithBuilder.Spec.Tag + "@sha256:just-built",
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
								LifecycleVersion: "some-version",
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
								Name:      imageName + "-build-2",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "2",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: builderName,
									buildapi.BuilderKindAnnotation: buildapi.BuilderKind,
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache:    &buildapi.BuildCacheConfig{},
								RunImage: builderRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-2"),
									},
									LatestBuildRef:             "image-name-build-2",
									LatestBuildReason:          "BUILDPACK",
									LatestBuildImageGeneration: originalGeneration,
									LatestImage:                imageWithBuilder.Spec.Tag + "@sha256:just-built",
									BuildCounter:               2,
								},
							},
						},
					},
				})
			})

			it("schedules a build when the builder stack is updated", func() {
				imageWithBuilder.Status.BuildCounter = 1
				imageWithBuilder.Status.LatestBuildRef = "image-name-build-1"
				const updatedBuilderImage = "some/builder@sha256:updated"
				const updatedBuilderRunImage = "gcr.io/test-project/install/run@sha256:01ea3600f15a73f0ad445351c681eb0377738f5964cbcd2bab0cfec9ca891a08"
				updatedRunImage := buildapi.BuildSpecImage{
					Image: updatedBuilderRunImage,
				}

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						&buildapi.Builder{
							ObjectMeta: metav1.ObjectMeta{
								Name:      builderName,
								Namespace: namespace,
							},
							TypeMeta: metav1.TypeMeta{
								Kind: buildapi.BuilderKind,
							},
							Status: buildapi.BuilderStatus{
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
										{
											Type:   buildapi.ConditionUpToDate,
											Status: corev1.ConditionTrue,
										},
									},
								},
								LatestImage: updatedBuilderImage,
								Stack: corev1alpha1.BuildStack{
									RunImage: updatedBuilderRunImage,
									ID:       "io.buildpacks.stacks.bionic",
								},
								Lifecycle: buildapi.ResolvedClusterLifecycle{
									Version: "some-version",
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
								Name:      "image-name-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: imageWithBuilder.Spec.Tag + "@sha256:just-built",
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
								LifecycleVersion: "some-version",
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
								Name:      imageName + "-build-2",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "2",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: builderName,
									buildapi.BuilderKindAnnotation: buildapi.BuilderKind,
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache:    &buildapi.BuildCacheConfig{},
								RunImage: updatedRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-2"),
									},
									LatestBuildRef:             "image-name-build-2",
									LatestBuildImageGeneration: originalGeneration,
									LatestBuildReason:          buildapi.BuildReasonStack,
									LatestImage:                imageWithBuilder.Spec.Tag + "@sha256:just-built",
									BuildCounter:               2,
								},
							},
						},
					},
				})
			})

			it("schedules a build when the builder lifecycle is updated", func() {
				imageWithBuilder.Status.BuildCounter = 1
				imageWithBuilder.Status.LatestBuildRef = "image-name-build-1"
				const updatedBuilderImage = "some/builder@sha256:updated"

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						&buildapi.Builder{
							ObjectMeta: metav1.ObjectMeta{
								Name:      builderName,
								Namespace: namespace,
							},
							TypeMeta: metav1.TypeMeta{
								Kind: buildapi.BuilderKind,
							},
							Status: buildapi.BuilderStatus{
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionReady,
											Status: corev1.ConditionTrue,
										},
										{
											Type:   buildapi.ConditionUpToDate,
											Status: corev1.ConditionTrue,
										},
									},
								},
								LatestImage: updatedBuilderImage,
								Stack: corev1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								Lifecycle: buildapi.ResolvedClusterLifecycle{
									Version: "some-new-version",
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
								Name:      "image-name-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: imageWithBuilder.Spec.Tag + "@sha256:just-built",
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
								LifecycleVersion: "some-version",
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
								Name:      imageName + "-build-2",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "2",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: builderName,
									buildapi.BuilderKindAnnotation: buildapi.BuilderKind,
									buildapi.BuildReasonAnnotation: buildapi.BuildReasonLifecycle,
									buildapi.BuildChangesAnnotation: testhelpers.CompactJSON(`
[
  {
    "reason": "LIFECYCLE",
    "old": "some-version",
    "new": "some-new-version"
  }
]`),
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: updatedBuilderImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache:    &buildapi.BuildCacheConfig{},
								RunImage: builderRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-2"),
									},
									LatestBuildRef:             "image-name-build-2",
									LatestBuildReason:          "LIFECYCLE",
									LatestBuildImageGeneration: originalGeneration,
									LatestImage:                imageWithBuilder.Spec.Tag + "@sha256:just-built",
									BuildCounter:               2,
								},
							},
						},
					},
				})
			})

			it("schedules a build with previous build's LastBuild if the last build failed", func() {
				imageWithBuilder.Status.BuildCounter = 2
				imageWithBuilder.Status.LatestBuildRef = "image-name-build200001"

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-1",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "2",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: "old-service-account",
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      "out-of-date-git-url",
										Revision: "out-of-date-git-revision",
									},
								},
								LastBuild: &buildapi.LastBuild{
									Image:   imageWithBuilder.Spec.Tag + "@sha256:from-build-before-this-build",
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
								Name:      imageName + "-build-3",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel:     "3",
									buildapi.ImageLabel:           imageName,
									buildapi.ImageGenerationLabel: generation(imageWithBuilder),
									someLabelKey:                  someValueToPassThrough,
								},
								Annotations: map[string]string{
									buildapi.BuilderNameAnnotation: builderName,
									buildapi.BuilderKindAnnotation: buildapi.BuilderKind,
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
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
								Cache:    &buildapi.BuildCacheConfig{},
								RunImage: builderRunImage,
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
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionBuildExecuting("image-name-build-3"),
									},
									LatestBuildRef:             "image-name-build-3",
									LatestBuildReason:          "COMMIT,CONFIG",
									LatestBuildImageGeneration: originalGeneration,
									BuildCounter:               3,
								},
							},
						},
					},
				})
			})

			it("does not schedule a build if the previous build is running and updates image status with build status", func() {
				imageWithBuilder.Status.BuildCounter = 1
				imageWithBuilder.Status.LatestBuildRef = "image-name-build-1"

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "image-name-build-100001",
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: "old-service-account",
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
											Type:    corev1alpha1.ConditionSucceeded,
											Status:  corev1.ConditionUnknown,
											Message: "Some build message",
										},
									},
								},
							},
						},
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:    corev1alpha1.ConditionReady,
												Status:  corev1.ConditionUnknown,
												Reason:  "BuildRunning",
												Message: "Some build message",
											},
											{
												Type:   buildapi.ConditionBuilderReady,
												Status: corev1.ConditionTrue,
												Reason: buildapi.BuilderReady,
											},
											{
												Type:   buildapi.ConditionBuilderUpToDate,
												Status: corev1.ConditionTrue,
												Reason: buildapi.BuilderUpToDate,
											},
										},
									},
									LatestBuildRef: "image-name-build-1",
									BuildCounter:   1,
								},
							},
						},
					},
				})
			})

			it("does not schedule a build if the previous build spec matches the current desired spec", func() {
				imageWithBuilder.Status.BuildCounter = 1
				imageWithBuilder.Status.LatestBuildRef = "image-name-build-1"
				imageWithBuilder.Status.LatestImage = "some/image@sha256:ad3f454c"
				imageWithBuilder.Status.Conditions = conditionReady()
				imageWithBuilder.Status.LatestStack = "io.buildpacks.stacks.bionic"

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						sourceResolver,
						&buildapi.Build{
							ObjectMeta: metav1.ObjectMeta{
								Name:      imageWithBuilder.Status.LatestBuildRef,
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*kmeta.NewControllerRef(imageWithBuilder),
								},
								Labels: map[string]string{
									buildapi.BuildNumberLabel: "1",
									buildapi.ImageLabel:       imageName,
								},
							},
							Spec: buildapi.BuildSpec{
								Tags: []string{imageWithBuilder.Spec.Tag},
								Builder: corev1alpha1.BuildBuilderSpec{
									Image: builder.Status.LatestImage,
								},
								ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
								Source: corev1alpha1.SourceConfig{
									Git: &corev1alpha1.Git{
										URL:      sourceResolver.Status.Source.Git.URL,
										Revision: sourceResolver.Status.Source.Git.Revision,
									},
								},
							},
							Status: buildapi.BuildStatus{
								LatestImage: imageWithBuilder.Status.LatestImage,
								Stack: corev1alpha1.BuildStack{
									RunImage: "some/run@sha256:67e3de2af270bf09c02e9a644aeb7e87e6b3c049abe6766bf6b6c3728a83e7fb",
									ID:       "io.buildpacks.stacks.bionic",
								},
								LifecycleVersion: "some-version",
								Status: corev1alpha1.Status{
									Conditions: corev1alpha1.Conditions{
										{
											Type:   corev1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
											Reason: image.UpToDateReason,
										},
										{
											Type:   buildapi.ConditionBuilderReady,
											Status: corev1.ConditionTrue,
											Reason: buildapi.BuilderReady,
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
				imageWithBuilder.Status.BuildCounter = 1
				imageWithBuilder.Status.LatestBuildRef = "image-name-build-1"
				imageWithBuilder.Status.LatestImage = "some/image@some-old-sha"
				imageWithBuilder.Status.LatestStack = "io.buildpacks.stacks.bionic"

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: runtimeObjects(
						successfulBuilds(imageWithBuilder, sourceResolver, 1),
						imageWithBuilder,
						builder,
						sourceResolver,
					),
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionTrue,
												Reason: image.UpToDateReason,
											},
											{
												Type:   buildapi.ConditionBuilderReady,
												Status: corev1.ConditionTrue,
												Reason: buildapi.BuilderReady,
											},
											{
												Type:   buildapi.ConditionBuilderUpToDate,
												Status: corev1.ConditionTrue,
												Reason: buildapi.BuilderUpToDate,
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
				imageWithBuilder.Status.BuildCounter = 1
				imageWithBuilder.Status.LatestBuildRef = "image-name-build-1"
				imageWithBuilder.Status.LatestImage = "some/image@some-old-sha"
				imageWithBuilder.Status.LatestStack = "io.buildpacks.stacks.bionic"

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: runtimeObjects(
						successfulBuilds(imageWithBuilder, sourceResolver, 1),
						imageWithBuilder,
						builder,
						unresolvedSourceResolver(imageWithBuilder),
					),
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions:         conditionReadyUnknown(),
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

			it("reports false when last build was successful and builder is not ready", func() {
				imageWithBuilder.Status.BuildCounter = 1
				imageWithBuilder.Status.LatestBuildRef = "image-name-build-1"
				imageWithBuilder.Status.LatestImage = "some/image@some-old-sha"
				imageWithBuilder.Status.LatestStack = "io.buildpacks.stacks.bionic"

				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: runtimeObjects(
						successfulBuilds(imageWithBuilder, sourceResolver, 1),
						imageWithBuilder,
						sourceResolver,
						notReadyBuilder(builder),
					),
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:    corev1alpha1.ConditionReady,
												Status:  corev1.ConditionFalse,
												Reason:  buildapi.BuilderNotReady,
												Message: "Error: Builder 'builder-name' is not ready in namespace 'some-namespace'",
											},
											{
												Type:    buildapi.ConditionBuilderReady,
												Status:  corev1.ConditionFalse,
												Reason:  buildapi.BuilderNotReady,
												Message: "Error: Builder 'builder-name' is not ready in namespace 'some-namespace'",
											},
											{
												Type:    buildapi.ConditionBuilderUpToDate,
												Status:  corev1.ConditionFalse,
												Reason:  buildapi.BuilderNotUpToDate,
												Message: "Builder is not up to date. The latest stack and buildpacks may not be in use.",
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

			it("includes failed builds status in not ready condition", func() {
				imageWithBuilder.Status.BuildCounter = 1
				failureMessage := "something went wrong"
				sourceResolver := resolvedSourceResolver(imageWithBuilder)
				failedBuild := &buildapi.Build{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-build-%d", imageWithBuilder.Name, 1),
						Namespace: imageWithBuilder.Namespace,
						OwnerReferences: []metav1.OwnerReference{
							*kmeta.NewControllerRef(imageWithBuilder),
						},
						Labels: map[string]string{
							buildapi.BuildNumberLabel: "1",
							buildapi.ImageLabel:       imageWithBuilder.Name,
						},
						CreationTimestamp: metav1.NewTime(time.Now().Add(time.Duration(1) * time.Minute)),
					},
					Spec: buildapi.BuildSpec{
						Tags: []string{imageWithBuilder.Spec.Tag},
						Builder: corev1alpha1.BuildBuilderSpec{
							Image: "builder-image/foo@sha256:112312",
						},
						ServiceAccountName: imageWithBuilder.Spec.ServiceAccountName,
						Source: corev1alpha1.SourceConfig{
							Git: &corev1alpha1.Git{
								URL:      sourceResolver.Status.Source.Git.URL,
								Revision: sourceResolver.Status.Source.Git.Revision,
							},
						},
					},
					Status: buildapi.BuildStatus{
						Status: corev1alpha1.Status{
							Conditions: corev1alpha1.Conditions{
								corev1alpha1.Condition{
									Type:    corev1alpha1.ConditionSucceeded,
									Status:  corev1.ConditionFalse,
									Message: failureMessage,
								},
							},
						},
					},
				}

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: runtimeObjects(
						[]runtime.Object{failedBuild},
						imageWithBuilder,
						builder,
						sourceResolver,
					),
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.Image{
								ObjectMeta: imageWithBuilder.ObjectMeta,
								Spec:       imageWithBuilder.Spec,
								Status: buildapi.ImageStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:    corev1alpha1.ConditionReady,
												Status:  corev1.ConditionFalse,
												Reason:  image.BuildFailedReason,
												Message: fmt.Sprintf("Error: Build '%s' in namespace '%s' failed: %s", failedBuild.Name, failedBuild.Namespace, failureMessage),
											},
											{
												Type:   buildapi.ConditionBuilderReady,
												Status: corev1.ConditionTrue,
												Reason: buildapi.BuilderReady,
											},
											{
												Type:   buildapi.ConditionBuilderUpToDate,
												Status: corev1.ConditionTrue,
												Reason: buildapi.BuilderUpToDate,
											},
										},
									},
									LatestBuildRef: "image-name-build-1",
									BuildCounter:   1,
								},
							},
						},
					},
				})
			})

			when("reconciling old builds", func() {
				it("deletes a failed build if more than the limit", func() {
					imageWithBuilder.Spec.FailedBuildHistoryLimit = limit(4)
					imageWithBuilder.Status.LatestBuildRef = "image-name-build-5"
					imageWithBuilder.Status.Conditions = conditionNotReady(imageWithBuilder)
					imageWithBuilder.Status.BuildCounter = 5
					sourceResolver := resolvedSourceResolver(imageWithBuilder)

					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: runtimeObjects(
							failedBuilds(imageWithBuilder, sourceResolver, 5),
							imageWithBuilder,
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
								Name: imageWithBuilder.Name + "-build-1", // first-build
							},
						},
					})
				})

				it("deletes a successful build if more than the limit", func() {
					imageWithBuilder.Spec.SuccessBuildHistoryLimit = limit(4)
					imageWithBuilder.Status.LatestBuildRef = "image-name-build-5"
					imageWithBuilder.Status.LatestImage = "some/image@sha256:build-5"
					imageWithBuilder.Status.LatestStack = "io.buildpacks.stacks.bionic"
					imageWithBuilder.Status.Conditions = conditionReady()
					imageWithBuilder.Status.BuildCounter = 5
					sourceResolver := resolvedSourceResolver(imageWithBuilder)

					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: runtimeObjects(
							successfulBuilds(imageWithBuilder, sourceResolver, 5),
							imageWithBuilder,
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
								Name: imageWithBuilder.Name + "-build-1", // first-build
							},
						},
					})
				})
			})
		})

		when("defaulting has not happened", func() {
			imageWithBuilder.Spec.FailedBuildHistoryLimit = nil
			imageWithBuilder.Spec.SuccessBuildHistoryLimit = nil

			it("sets the FailedBuildHistoryLimit and SuccessBuildHistoryLimit", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						imageWithBuilder,
						builder,
						clusterBuilder,
						unresolvedSourceResolver(imageWithBuilder),
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

func builderWithCondition(builder *buildapi.Builder, conditions ...corev1alpha1.Condition) runtime.Object {
	builder.Status.Conditions = corev1alpha1.Conditions{}

	for _, condition := range conditions {
		builder.Status.Conditions = append(builder.Status.Conditions, condition)
	}

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
				ServiceAccountName: image.Spec.ServiceAccountName,
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
				LifecycleVersion: "some-version",
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
			Type:    corev1alpha1.ConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  image.ResolverNotReadyReason,
			Message: "Error: SourceResolver 'image-name-source' is not ready",
		},
		{
			Type:   buildapi.ConditionBuilderReady,
			Status: corev1.ConditionTrue,
			Reason: buildapi.BuilderReady,
		},
		{
			Type:   buildapi.ConditionBuilderUpToDate,
			Status: corev1.ConditionTrue,
			Reason: buildapi.BuilderUpToDate,
		},
	}
}

func conditionBuildExecuting(buildName string) corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:    corev1alpha1.ConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  image.BuildRunningReason,
			Message: fmt.Sprintf("Build '%s' is executing", buildName),
		},
		{
			Type:   buildapi.ConditionBuilderReady,
			Status: corev1.ConditionTrue,
			Reason: buildapi.BuilderReady,
		},
		{
			Type:   buildapi.ConditionBuilderUpToDate,
			Status: corev1.ConditionTrue,
			Reason: buildapi.BuilderUpToDate,
		},
	}
}

func conditionReady() corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:   corev1alpha1.ConditionReady,
			Status: corev1.ConditionTrue,
			Reason: image.UpToDateReason,
		},
		{
			Type:   buildapi.ConditionBuilderReady,
			Status: corev1.ConditionTrue,
			Reason: buildapi.BuilderReady,
		},
		{
			Type:   buildapi.ConditionBuilderUpToDate,
			Status: corev1.ConditionTrue,
			Reason: buildapi.BuilderUpToDate,
		},
	}
}

func conditionNotReady(failedBuild *buildapi.Image) corev1alpha1.Conditions {
	return corev1alpha1.Conditions{
		{
			Type:    corev1alpha1.ConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  image.BuildFailedReason,
			Message: fmt.Sprintf("Error: Build '%s' in namespace '%s' failed: ", failedBuild.Status.LatestBuildRef, failedBuild.Namespace),
		},
		{
			Type:   buildapi.ConditionBuilderReady,
			Status: corev1.ConditionTrue,
			Reason: buildapi.BuilderReady,
		},
		{
			Type:   buildapi.ConditionBuilderUpToDate,
			Status: corev1.ConditionTrue,
			Reason: buildapi.BuilderUpToDate,
		},
	}
}
