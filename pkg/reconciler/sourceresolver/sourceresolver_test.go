package sourceresolver_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	rtesting "knative.dev/pkg/reconciler/testing"

	buildapi "github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/pivotal/kpack/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/kpack/pkg/reconciler/sourceresolver"
	"github.com/pivotal/kpack/pkg/reconciler/sourceresolver/sourceresolverfakes"
	"github.com/pivotal/kpack/pkg/reconciler/testhelpers"
)

func TestSourceResolver(t *testing.T) {
	spec.Run(t, "Source Resolver Reconciler", testSourceResolver)
}

func testSourceResolver(t *testing.T, when spec.G, it spec.S) {
	const (
		sourceResolverName       = "source-resolver-name"
		namespace                = "some-namespace"
		key                      = "some-namespace/source-resolver-name"
		originalGeneration int64 = 0
		serviceAccount           = "serviceAccount"
	)

	fakeGitResolver := &sourceresolverfakes.FakeResolver{}
	fakeBlobResolver := &sourceresolverfakes.FakeResolver{}
	fakeRegistryResolver := &sourceresolverfakes.FakeResolver{}
	fakeEnqueuer := &sourceresolverfakes.FakeEnqueuer{}

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList) {
			listers := testhelpers.NewListers(row.Objects)

			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			k8sfakeClient := k8sfake.NewSimpleClientset(listers.GetKubeObjects()...)

			eventRecorder := record.NewFakeRecorder(10)
			actionRecorderList := rtesting.ActionRecorderList{fakeClient, k8sfakeClient}
			eventList := rtesting.EventList{Recorder: eventRecorder}

			r := &sourceresolver.Reconciler{
				GitResolver:          fakeGitResolver,
				BlobResolver:         fakeBlobResolver,
				RegistryResolver:     fakeRegistryResolver,
				Enqueuer:             fakeEnqueuer,
				Client:               fakeClient,
				SourceResolverLister: listers.GetSourceResolverLister(),
			}

			rtesting.PrependGenerateNameReactor(&fakeClient.Fake)

			return r, actionRecorderList, eventList
		})

	when("#Reconcile", func() {
		when("a git based source config", func() {
			sourceResolver := &buildapi.SourceResolver{
				ObjectMeta: metav1.ObjectMeta{
					Name:       sourceResolverName,
					Namespace:  namespace,
					Generation: originalGeneration,
				},
				Spec: buildapi.SourceResolverSpec{
					ServiceAccountName: serviceAccount,
					Source: corev1alpha1.SourceConfig{
						Git: &corev1alpha1.Git{
							URL:      "https://github.com/build-me",
							Revision: "1234",
						},
					},
				},
			}

			resolvedSource := corev1alpha1.ResolvedSourceConfig{
				Git: &corev1alpha1.ResolvedGitSource{
					URL:      "https://example.com/something",
					Revision: "abcdef",
					Type:     corev1alpha1.Branch,
				},
			}

			fakeGitResolver.ResolveReturns(resolvedSource, nil)
			fakeGitResolver.CanResolveReturns(true)

			it("updates the observed generation", func() {
				sourceResolver := resolvedSourceResolver(sourceResolver, resolvedSource)
				sourceResolver.Generation = 2

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						sourceResolver,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.SourceResolver{
								ObjectMeta: sourceResolver.ObjectMeta,
								Spec:       sourceResolver.Spec,
								Status: buildapi.SourceResolverStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: 2,
										Conditions:         sourceResolver.Status.Conditions,
									},
									Source: sourceResolver.Status.Source,
								},
							},
						},
					},
				})
			})

			it("does not unnecessarily update the resource", func() {
				sourceResolver := resolvedSourceResolver(sourceResolver, resolvedSource)

				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						sourceResolver,
					},
					WantErr: false,
				})
			})

			when("a branch is the source", func() {
				resolvedSource := corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      "https://example.com/something",
						Revision: "abcdef",
						Type:     corev1alpha1.Branch,
					},
				}

				fakeGitResolver.ResolveReturns(resolvedSource, nil)
				fakeGitResolver.CanResolveReturns(true)

				it("resolves git with the resolved source resolver", func() {
					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: []runtime.Object{
							sourceResolver,
						},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &buildapi.SourceResolver{
									ObjectMeta: sourceResolver.ObjectMeta,
									Spec:       sourceResolver.Spec,
									Status: buildapi.SourceResolverStatus{
										Status: corev1alpha1.Status{
											ObservedGeneration: originalGeneration,
											Conditions: corev1alpha1.Conditions{
												{
													Type:   corev1alpha1.ConditionReady,
													Status: corev1.ConditionTrue,
												},
												{
													Type:   buildapi.ActivePolling,
													Status: corev1.ConditionTrue,
												},
											},
										},
										Source: corev1alpha1.ResolvedSourceConfig{
											Git: &corev1alpha1.ResolvedGitSource{
												URL:      "https://example.com/something",
												Revision: "abcdef",
												Type:     corev1alpha1.Branch,
											},
										},
									},
								},
							},
						},
					})

					require.Equal(t, 1, fakeEnqueuer.EnqueueCallCount())
					enquedSourceResolver := fakeEnqueuer.EnqueueArgsForCall(0)
					require.Equal(t, sourceResolver.Name, enquedSourceResolver.Name)
					require.Equal(t, sourceResolver.Namespace, enquedSourceResolver.Namespace)
				})
			})

			when("a specific commit sha is the source", func() {
				resolvedSource := corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      "https://example.com/something",
						Revision: "abcdef",
						Type:     corev1alpha1.Commit,
					},
				}

				fakeGitResolver.ResolveReturns(resolvedSource, nil)
				fakeGitResolver.CanResolveReturns(true)

				it("reconciles to ready and not active polling", func() {
					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: []runtime.Object{
							sourceResolver,
						},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &buildapi.SourceResolver{
									ObjectMeta: sourceResolver.ObjectMeta,
									Spec:       sourceResolver.Spec,
									Status: buildapi.SourceResolverStatus{
										Status: corev1alpha1.Status{
											ObservedGeneration: originalGeneration,
											Conditions: corev1alpha1.Conditions{
												{
													Type:   corev1alpha1.ConditionReady,
													Status: corev1.ConditionTrue,
												},
												{
													Type:   buildapi.ActivePolling,
													Status: corev1.ConditionFalse,
												},
											},
										},
										Source: corev1alpha1.ResolvedSourceConfig{
											Git: &corev1alpha1.ResolvedGitSource{
												URL:      "https://example.com/something",
												Revision: "abcdef",
												Type:     corev1alpha1.Commit,
											},
										},
									},
								},
							},
						},
					})

					require.Equal(t, 0, fakeEnqueuer.EnqueueCallCount())
				})
			})

			when("git resolves to unknown", func() {
				resolvedSource := corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      "https://example.com/something",
						Revision: "abcdef",
						Type:     corev1alpha1.Unknown,
					},
				}

				fakeGitResolver.ResolveReturns(resolvedSource, nil)
				fakeGitResolver.CanResolveReturns(true)

				it("saves unknown when source has not previously resolved", func() {
					sourceResolver.Generation = 1

					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: []runtime.Object{
							sourceResolver,
						},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &buildapi.SourceResolver{
									ObjectMeta: sourceResolver.ObjectMeta,
									Spec:       sourceResolver.Spec,
									Status: buildapi.SourceResolverStatus{
										Status: corev1alpha1.Status{
											ObservedGeneration: 1,
											Conditions: corev1alpha1.Conditions{
												{
													Type:   corev1alpha1.ConditionReady,
													Status: corev1.ConditionTrue,
												},
												{
													Type:   buildapi.ActivePolling,
													Status: corev1.ConditionFalse,
												},
											},
										},
										Source: corev1alpha1.ResolvedSourceConfig{
											Git: &corev1alpha1.ResolvedGitSource{
												URL:      "https://example.com/something",
												Revision: "abcdef",
												Type:     corev1alpha1.Unknown,
											},
										},
									},
								},
							},
						},
					})
				})

				it("ignores unknown when source has been previously resolved", func() {
					alreadyResolvedSource := corev1alpha1.ResolvedSourceConfig{
						Git: &corev1alpha1.ResolvedGitSource{
							URL:      "https://example.com/something",
							Revision: "abcdef",
							Type:     corev1alpha1.Commit,
						},
					}

					alreadyResolvedSourceResolver := sourceResolver.DeepCopy()
					alreadyResolvedSourceResolver = resolvedSourceResolver(alreadyResolvedSourceResolver, alreadyResolvedSource)
					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: []runtime.Object{
							sourceResolver,
						},
						WantErr: false,
					})
				})
			})
		})

		when("a blob based source config", func() {
			sourceResolver := &buildapi.SourceResolver{
				ObjectMeta: metav1.ObjectMeta{
					Name:       sourceResolverName,
					Namespace:  namespace,
					Generation: originalGeneration,
				},
				Spec: buildapi.SourceResolverSpec{
					ServiceAccountName: serviceAccount,
					Source: corev1alpha1.SourceConfig{
						Blob: &corev1alpha1.Blob{
							URL: "https://some-blobstore.example.com/some-blob",
						},
					},
				},
			}

			resolvedSource := corev1alpha1.ResolvedSourceConfig{
				Blob: &corev1alpha1.ResolvedBlobSource{
					URL: "https://some-blobstore.example.com/some-blob",
				},
			}

			fakeBlobResolver.ResolveReturns(resolvedSource, nil)
			fakeBlobResolver.CanResolveReturns(true)

			it("reconciles to ready and not active polling", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						sourceResolver,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.SourceResolver{
								ObjectMeta: sourceResolver.ObjectMeta,
								Spec:       sourceResolver.Spec,
								Status: buildapi.SourceResolverStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionTrue,
											},
											{
												Type:   buildapi.ActivePolling,
												Status: corev1.ConditionFalse,
											},
										},
									},
									Source: corev1alpha1.ResolvedSourceConfig{
										Blob: &corev1alpha1.ResolvedBlobSource{
											URL: "https://some-blobstore.example.com/some-blob",
										},
									},
								},
							},
						},
					},
				})
			})
		})

		when("a registry based source config", func() {
			sourceResolver := &buildapi.SourceResolver{
				ObjectMeta: metav1.ObjectMeta{
					Name:       sourceResolverName,
					Namespace:  namespace,
					Generation: originalGeneration,
				},
				Spec: buildapi.SourceResolverSpec{
					ServiceAccountName: serviceAccount,
					Source: corev1alpha1.SourceConfig{
						Registry: &corev1alpha1.Registry{
							Image: "some-registry.io/some-image@sha256:abcdef123456",
						},
					},
				},
			}

			resolvedSource := corev1alpha1.ResolvedSourceConfig{
				Registry: &corev1alpha1.ResolvedRegistrySource{
					Image: "some-registry.io/some-image@sha256:abcdef123456",
				},
			}

			fakeRegistryResolver.ResolveReturns(resolvedSource, nil)
			fakeRegistryResolver.CanResolveReturns(true)

			it("reconciles to ready and not active polling", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						sourceResolver,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &buildapi.SourceResolver{
								ObjectMeta: sourceResolver.ObjectMeta,
								Spec:       sourceResolver.Spec,
								Status: buildapi.SourceResolverStatus{
									Status: corev1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: corev1alpha1.Conditions{
											{
												Type:   corev1alpha1.ConditionReady,
												Status: corev1.ConditionTrue,
											},
											{
												Type:   buildapi.ActivePolling,
												Status: corev1.ConditionFalse,
											},
										},
									},
									Source: corev1alpha1.ResolvedSourceConfig{
										Registry: &corev1alpha1.ResolvedRegistrySource{
											Image: "some-registry.io/some-image@sha256:abcdef123456",
										},
									},
								},
							},
						},
					},
				})
			})
		})
	})
}

func resolvedSourceResolver(sourceResolver *buildapi.SourceResolver, resolvedSource corev1alpha1.ResolvedSourceConfig) *buildapi.SourceResolver {
	sourceResolver.ResolvedSource(resolvedSource)
	return sourceResolver
}
