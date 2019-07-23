package image_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/kmeta"
	rtesting "github.com/knative/pkg/reconciler/testing"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/image"
)

func TestImageReconciler(t *testing.T) {
	spec.Run(t, "Image Reconciler", testImageReconciler)
}

func testImageReconciler(t *testing.T, when spec.G, it spec.S) {

	const (
		imageName                    = "image-name"
		builderName                  = "builder-name"
		serviceAccount               = "service-account"
		namespace                    = "some-namespace"
		key                          = "some-namespace/image-name"
		someLabelKey                 = "some/label"
		someValueToPassThrough       = "to-pass-through"
		originalGeneration     int64 = 0
	)
	var (
		fakeTracker = fakeTracker{}
	)

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList, reporter *rtesting.FakeStatsReporter) {
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
				BuilderLister:        listers.GetBuilderLister(),
				SourceResolverLister: listers.GetSourceResolverLister(),
				PvcLister:            listers.GetPersistentVolumeClaimLister(),
				Tracker:              fakeTracker,
				K8sClient:            k8sfakeClient,
			}

			rtesting.PrependGenerateNameReactor(&fakeClient.Fake)

			return r, actionRecorderList, eventList, &rtesting.FakeStatsReporter{}
		})

	image := &v1alpha1.Image{
		ObjectMeta: v1.ObjectMeta{
			Name:      imageName,
			Namespace: namespace,
			Labels: map[string]string{
				someLabelKey: someValueToPassThrough,
			},
		},
		Spec: v1alpha1.ImageSpec{
			Image:          "some/image",
			ServiceAccount: serviceAccount,
			BuilderRef:     builderName,
			Source: v1alpha1.Source{
				Git: v1alpha1.Git{
					URL:      "https://some.git/url",
					Revision: "1234567",
				},
			},
			DisableAdditionalImageNames: true,
		},
		Status: v1alpha1.ImageStatus{
			Status: duckv1alpha1.Status{
				ObservedGeneration: originalGeneration,
			},
		},
	}

	builder := &v1alpha1.Builder{
		ObjectMeta: v1.ObjectMeta{
			Name:      builderName,
			Namespace: namespace,
		},
		Spec: v1alpha1.BuilderSpec{
			Image: "some/builder@sha256acf123",
		},
		Status: v1alpha1.BuilderStatus{
			BuilderMetadata: v1alpha1.BuildpackMetadataList{
				{
					ID:      "buildpack.version",
					Version: "version",
				},
			},
		},
	}

	when("Reconcile", func() {
		it("updates observed generation after processing an update", func() {
			const updatedGeneration int64 = 1
			image.ObjectMeta.Generation = updatedGeneration

			rt.Test(rtesting.TableRow{
				Key: key,
				Objects: []runtime.Object{
					image,
					builder,
					unresolvedSourceResolver(image),
				},
				WantErr: false,
				WantStatusUpdates: []clientgotesting.UpdateActionImpl{
					{
						Object: &v1alpha1.Image{
							ObjectMeta: image.ObjectMeta,
							Spec:       image.Spec,
							Status: v1alpha1.ImageStatus{
								Status: duckv1alpha1.Status{
									ObservedGeneration: updatedGeneration,
								},
								LatestBuildRef: "",
								BuildCounter:   0,
								BuildCacheName: "",
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
					unresolvedSourceResolver(image),
				},
				WantErr: false,
			})

			require.True(t, fakeTracker.IsTracking(builder.Ref(), image))
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
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
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
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
									},
									LatestBuildRef: "",
									BuildCounter:   0,
									BuildCacheName: image.CacheName(),
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
							ObjectMeta: v1.ObjectMeta{
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
								ObjectMeta: v1.ObjectMeta{
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
								ObjectMeta: v1.ObjectMeta{
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
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
									},
									LatestBuildRef: "",
									BuildCounter:   0,
									BuildCacheName: "",
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
									v1alpha1.BuildNumberLabel: "1",
									v1alpha1.ImageLabel:       imageName,
									someLabelKey:              someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonConfig,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Image:          image.Spec.Image,
								Builder:        builder.Spec.Image,
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      sourceResolver.Status.ResolvedSource.Git.URL,
										Revision: sourceResolver.Status.ResolvedSource.Git.Revision,
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
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
									},
									LatestBuildRef: "image-name-build-1-00001", //GenerateNameReactor
									BuildCounter:   1,
									BuildCacheName: "",
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
									v1alpha1.BuildNumberLabel: "1",
									v1alpha1.ImageLabel:       imageName,
									someLabelKey:              someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonConfig,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Image:          image.Spec.Image,
								Builder:        builder.Spec.Image,
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      sourceResolver.Status.ResolvedSource.Git.URL,
										Revision: sourceResolver.Status.ResolvedSource.Git.Revision,
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
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
									},
									LatestBuildRef: "image-name-build-1-00001", //GenerateNameReactor
									BuildCounter:   1,
									BuildCacheName: image.CacheName(),
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
								Image:          image.Spec.Image,
								Builder:        builder.Spec.Image,
								ServiceAccount: "old-service-account",
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      "out-of-date-git-url",
										Revision: "out-of-date-git-revision",
									},
								},
							},
							Status: v1alpha1.BuildStatus{
								SHA: "sha256:ad3f454c",
								Status: duckv1alpha1.Status{
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
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
									v1alpha1.BuildNumberLabel: "2",
									v1alpha1.ImageLabel:       imageName,
									someLabelKey:              someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: strings.Join([]string{v1alpha1.BuildReasonConfig, v1alpha1.BuildReasonCommit}, ","),
								},
							},
							Spec: v1alpha1.BuildSpec{
								Image:          image.Spec.Image,
								Builder:        builder.Spec.Image,
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      sourceResolver.Status.ResolvedSource.Git.URL,
										Revision: sourceResolver.Status.ResolvedSource.Git.Revision,
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
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
									},
									LatestBuildRef: "image-name-build-2-00001", //GenerateNameReactor
									LatestImage:    "some/image@sha256:ad3f454c",
									BuildCounter:   2,
									BuildCacheName: "",
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
				sourceResolver.ResolvedGitSource(v1alpha1.ResolvedGitSource{
					URL:      image.Spec.Source.Git.URL,
					Revision: "new-commit",
					Type:     v1alpha1.Branch,
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
								Image:          image.Spec.Image,
								Builder:        builder.Spec.Image,
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      image.Spec.Source.Git.URL,
										Revision: image.Spec.Source.Git.Revision,
									},
								},
							},
							Status: v1alpha1.BuildStatus{
								SHA: "sha256:ad3f454c",
								Status: duckv1alpha1.Status{
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
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
									v1alpha1.BuildNumberLabel: "2",
									v1alpha1.ImageLabel:       imageName,
									someLabelKey:              someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonCommit,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Image:          image.Spec.Image,
								Builder:        builder.Spec.Image,
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      sourceResolver.Status.ResolvedSource.Git.URL,
										Revision: sourceResolver.Status.ResolvedSource.Git.Revision,
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
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
									},
									LatestBuildRef: "image-name-build-2-00001", //GenerateNameReactor
									LatestImage:    "some/image@sha256:ad3f454c",
									BuildCounter:   2,
									BuildCacheName: "",
								},
							},
						},
					},
				})
			})

			it("schedules a build when the builder buildpacks are updated", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-1-00001"

				sourceResolver := resolvedSourceResolver(image)
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						image,
						&v1alpha1.Builder{
							ObjectMeta: v1.ObjectMeta{
								Name:      builderName,
								Namespace: namespace,
							},
							Spec: v1alpha1.BuilderSpec{
								Image: "some/builder@sha256acf123",
							},
							Status: v1alpha1.BuilderStatus{
								BuilderMetadata: v1alpha1.BuildpackMetadataList{
									{
										ID:      "io.buildpack",
										Version: "newversion",
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
								Image:          image.Spec.Image,
								Builder:        builder.Spec.Image,
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      sourceResolver.Status.ResolvedSource.Git.URL,
										Revision: sourceResolver.Status.ResolvedSource.Git.Revision,
									},
								},
							},
							Status: v1alpha1.BuildStatus{
								SHA: "sha256:ad3f454c",
								Status: duckv1alpha1.Status{
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
											Status: corev1.ConditionTrue,
										},
									},
								},
								BuildMetadata: v1alpha1.BuildpackMetadataList{
									{
										ID:      "io.buildpack",
										Version: "oldversion",
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
									v1alpha1.BuildNumberLabel: "2",
									v1alpha1.ImageLabel:       imageName,
									someLabelKey:              someValueToPassThrough,
								},
								Annotations: map[string]string{
									v1alpha1.BuildReasonAnnotation: v1alpha1.BuildReasonBuildpack,
								},
							},
							Spec: v1alpha1.BuildSpec{
								Image:          image.Spec.Image,
								Builder:        builder.Spec.Image,
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      sourceResolver.Status.ResolvedSource.Git.URL,
										Revision: sourceResolver.Status.ResolvedSource.Git.Revision,
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
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
									},
									LatestBuildRef: "image-name-build-2-00001", //GenerateNameReactor
									LatestImage:    "some/image@sha256:ad3f454c",
									BuildCounter:   2,
									BuildCacheName: "",
								},
							},
						},
					},
				})
			})

			it("does not schedule a build if the previous build is running", func() {
				image.Generation = 2
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
								Image:          image.Spec.Image,
								Builder:        builder.Spec.Image,
								ServiceAccount: "old-service-account",
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      "out-of-date-git-url",
										Revision: "out-of-date-git-revision",
									},
								},
							},
							Status: v1alpha1.BuildStatus{
								Status: duckv1alpha1.Status{
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
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
								Image:          image.Spec.Image,
								Builder:        builder.Spec.Image,
								ServiceAccount: image.Spec.ServiceAccount,
								Source: v1alpha1.Source{
									Git: v1alpha1.Git{
										URL:      sourceResolver.Status.ResolvedSource.Git.URL,
										Revision: sourceResolver.Status.ResolvedSource.Git.Revision,
									},
								},
							},
							Status: v1alpha1.BuildStatus{
								SHA: "sha256:ad3f454c",
								Status: duckv1alpha1.Status{
									Conditions: duckv1alpha1.Conditions{
										{
											Type:   duckv1alpha1.ConditionSucceeded,
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

			when("reconciling old builds", func() {

				it("deletes a failed build if more than the limit", func() {
					image.Spec.FailedBuildHistoryLimit = limit(4)
					image.Status.LatestBuildRef = "image-name-build-5"
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
								Name: image.Name + "-build-1", //first-build
							},
						},
					})
				})

				it("deletes a successful build if more than the limit", func() {
					image.Spec.SuccessBuildHistoryLimit = limit(4)
					image.Status.LatestBuildRef = "image-name-build-5"
					image.Status.LatestImage = "some/image@sha256:ad3f454c"
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
								Name: image.Name + "-build-1", //first-build
							},
						},
					})
				})
			})

			it("updates the last successful build on the image when the last build is successful", func() {
				image.Status.BuildCounter = 1
				image.Status.LatestBuildRef = "image-name-build-1"
				image.Status.LatestImage = "some/image@some-old-sha"

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
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
									},
									LatestBuildRef: "image-name-build-1",
									LatestImage:    "some/image@sha256:ad3f454c",
									BuildCounter:   1,
									BuildCacheName: "",
								},
							},
						},
					},
				})
			})
		})
	})
}

func resolvedSourceResolver(image *v1alpha1.Image) *v1alpha1.SourceResolver {
	sr := image.SourceResolver()
	sr.ResolvedGitSource(v1alpha1.ResolvedGitSource{
		URL:      image.Spec.Source.Git.URL + "-resolved",
		Revision: image.Spec.Source.Git.Revision + "-resolved",
		Type:     v1alpha1.Branch,
	})
	return sr
}

func unresolvedSourceResolver(image *v1alpha1.Image) *v1alpha1.SourceResolver {
	return image.SourceResolver()
}

func failedBuilds(image *v1alpha1.Image, sourceResolver *v1alpha1.SourceResolver, count int) []runtime.Object {
	return builds(image, sourceResolver, count, duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionFalse,
	})
}

func successfulBuilds(image *v1alpha1.Image, sourceResolver *v1alpha1.SourceResolver, count int) []runtime.Object {
	return builds(image, sourceResolver, count, duckv1alpha1.Condition{
		Type:   duckv1alpha1.ConditionSucceeded,
		Status: corev1.ConditionTrue,
	})
}

func builds(image *v1alpha1.Image, sourceResolver *v1alpha1.SourceResolver, count int, condition duckv1alpha1.Condition) []runtime.Object {
	var builds []runtime.Object

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
				Image:          image.Spec.Image,
				Builder:        "some/builder",
				ServiceAccount: image.Spec.ServiceAccount,
				Source: v1alpha1.Source{
					Git: v1alpha1.Git{
						URL:      sourceResolver.Status.ResolvedSource.Git.URL,
						Revision: sourceResolver.Status.ResolvedSource.Git.Revision,
					},
				},
			},
			Status: v1alpha1.BuildStatus{
				SHA: "sha256:ad3f454c",
				Status: duckv1alpha1.Status{
					Conditions: duckv1alpha1.Conditions{
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
