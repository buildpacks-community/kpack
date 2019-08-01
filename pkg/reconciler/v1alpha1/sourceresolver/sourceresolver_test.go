package sourceresolver_test

import (
	"testing"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/controller"
	rtesting "github.com/knative/pkg/reconciler/testing"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/reconciler/testhelpers"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/sourceresolver"
	"github.com/pivotal/build-service-system/pkg/reconciler/v1alpha1/sourceresolver/sourceresolverfakes"
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
	fakeEnqueuer := &sourceresolverfakes.FakeEnqueuer{}

	rt := testhelpers.ReconcilerTester(t,
		func(t *testing.T, row *rtesting.TableRow) (reconciler controller.Reconciler, lists rtesting.ActionRecorderList, list rtesting.EventList, reporter *rtesting.FakeStatsReporter) {
			listers := testhelpers.NewListers(row.Objects)

			fakeClient := fake.NewSimpleClientset(listers.BuildServiceObjects()...)
			k8sfakeClient := k8sfake.NewSimpleClientset(listers.GetKubeObjects()...)

			eventRecorder := record.NewFakeRecorder(10)
			actionRecorderList := rtesting.ActionRecorderList{fakeClient, k8sfakeClient}
			eventList := rtesting.EventList{Recorder: eventRecorder}

			r := &sourceresolver.Reconciler{
				GitResolver:          fakeGitResolver,
				BlobResolver:         fakeBlobResolver,
				Enqueuer:             fakeEnqueuer,
				Client:               fakeClient,
				SourceResolverLister: listers.GetSourceResolverLister(),
			}

			rtesting.PrependGenerateNameReactor(&fakeClient.Fake)

			return r, actionRecorderList, eventList, &rtesting.FakeStatsReporter{}
		})

	when("#Reconcile", func() {
		when("a git based source config", func() {
			sourceResolver := &v1alpha1.SourceResolver{
				ObjectMeta: v1.ObjectMeta{
					Name:       sourceResolverName,
					Namespace:  namespace,
					Generation: originalGeneration,
				},
				Spec: v1alpha1.SourceResolverSpec{
					ServiceAccount: serviceAccount,
					Source: v1alpha1.Source{
						Git: &v1alpha1.Git{
							URL:      "https://github.com/build-me",
							Revision: "1234",
						},
					},
				},
			}

			resolvedSource := v1alpha1.ResolvedSource{
				Git: &v1alpha1.ResolvedGitSource{
					URL:      "https://example.com/something",
					Revision: "abcdef",
					Type:     v1alpha1.Branch,
				},
			}

			fakeGitResolver.ResolveReturns(resolvedSource, nil)

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
							Object: &v1alpha1.SourceResolver{
								ObjectMeta: sourceResolver.ObjectMeta,
								Spec:       sourceResolver.Spec,
								Status: v1alpha1.SourceResolverStatus{
									Status: duckv1alpha1.Status{
										ObservedGeneration: 2,
										Conditions:         sourceResolver.Status.Conditions,
									},
									ResolvedSource: sourceResolver.Status.ResolvedSource,
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
				resolvedSource := v1alpha1.ResolvedSource{
					Git: &v1alpha1.ResolvedGitSource{
						URL:      "https://example.com/something",
						Revision: "abcdef",
						Type:     v1alpha1.Branch,
					},
				}

				fakeGitResolver.ResolveReturns(resolvedSource, nil)

				it("resolves git with the resolved source resolver", func() {
					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: []runtime.Object{
							sourceResolver,
						},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &v1alpha1.SourceResolver{
									ObjectMeta: sourceResolver.ObjectMeta,
									Spec:       sourceResolver.Spec,
									Status: v1alpha1.SourceResolverStatus{
										Status: duckv1alpha1.Status{
											ObservedGeneration: originalGeneration,
											Conditions: duckv1alpha1.Conditions{
												{
													Type:   duckv1alpha1.ConditionReady,
													Status: corev1.ConditionTrue,
												},
												{
													Type:   v1alpha1.ActivePolling,
													Status: corev1.ConditionTrue,
												},
											},
										},
										ResolvedSource: v1alpha1.ResolvedSource{
											Git: &v1alpha1.ResolvedGitSource{
												URL:      "https://example.com/something",
												Revision: "abcdef",
												Type:     v1alpha1.Branch,
											},
										},
									},
								},
							},
						},
					})

					enquedSourceResolver := fakeEnqueuer.EnqueueArgsForCall(0)
					require.Equal(t, sourceResolver.Name, enquedSourceResolver.Name)
					require.Equal(t, sourceResolver.Namespace, enquedSourceResolver.Namespace)
				})
			})

			when("a specific commit sha is the source", func() {
				resolvedSource := v1alpha1.ResolvedSource{
					Git: &v1alpha1.ResolvedGitSource{
						URL:      "https://example.com/something",
						Revision: "abcdef",
						Type:     v1alpha1.Commit,
					},
				}

				fakeGitResolver.ResolveReturns(resolvedSource, nil)

				it("reconciles to ready and not active polling", func() {
					rt.Test(rtesting.TableRow{
						Key: key,
						Objects: []runtime.Object{
							sourceResolver,
						},
						WantErr: false,
						WantStatusUpdates: []clientgotesting.UpdateActionImpl{
							{
								Object: &v1alpha1.SourceResolver{
									ObjectMeta: sourceResolver.ObjectMeta,
									Spec:       sourceResolver.Spec,
									Status: v1alpha1.SourceResolverStatus{
										Status: duckv1alpha1.Status{
											ObservedGeneration: originalGeneration,
											Conditions: duckv1alpha1.Conditions{
												{
													Type:   duckv1alpha1.ConditionReady,
													Status: corev1.ConditionTrue,
												},
												{
													Type:   v1alpha1.ActivePolling,
													Status: corev1.ConditionFalse,
												},
											},
										},
										ResolvedSource: v1alpha1.ResolvedSource{
											Git: &v1alpha1.ResolvedGitSource{
												URL:      "https://example.com/something",
												Revision: "abcdef",
												Type:     v1alpha1.Commit,
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
				resolvedSource := v1alpha1.ResolvedSource{
					Git: &v1alpha1.ResolvedGitSource{
						URL:      "https://example.com/something",
						Revision: "abcdef",
						Type:     v1alpha1.Unknown,
					},
				}

				fakeGitResolver.ResolveReturns(resolvedSource, nil)

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
								Object: &v1alpha1.SourceResolver{
									ObjectMeta: sourceResolver.ObjectMeta,
									Spec:       sourceResolver.Spec,
									Status: v1alpha1.SourceResolverStatus{
										Status: duckv1alpha1.Status{
											ObservedGeneration: 1,
											Conditions: duckv1alpha1.Conditions{
												{
													Type:   duckv1alpha1.ConditionReady,
													Status: corev1.ConditionTrue,
												},
												{
													Type:   v1alpha1.ActivePolling,
													Status: corev1.ConditionFalse,
												},
											},
										},
										ResolvedSource: v1alpha1.ResolvedSource{
											Git: &v1alpha1.ResolvedGitSource{
												URL:      "https://example.com/something",
												Revision: "abcdef",
												Type:     v1alpha1.Unknown,
											},
										},
									},
								},
							},
						},
					})
				})

				it("ignores unknown when source has been previously resolved", func() {
					alreadyResolvedSource := v1alpha1.ResolvedSource{
						Git: &v1alpha1.ResolvedGitSource{
							URL:      "https://example.com/something",
							Revision: "abcdef",
							Type:     v1alpha1.Commit,
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
			sourceResolver := &v1alpha1.SourceResolver{
				ObjectMeta: v1.ObjectMeta{
					Name:       sourceResolverName,
					Namespace:  namespace,
					Generation: originalGeneration,
				},
				Spec: v1alpha1.SourceResolverSpec{
					ServiceAccount: serviceAccount,
					Source: v1alpha1.Source{
						Blob: &v1alpha1.Blob{
							URL: "https://some-blobstore.example.com/some-blob",
						},
					},
				},
			}

			resolvedSource := v1alpha1.ResolvedSource{
				Blob: &v1alpha1.ResolvedBlobSource{
					URL: "https://some-blobstore.example.com/some-blob",
				},
			}

			fakeBlobResolver.ResolveReturns(resolvedSource, nil)

			it("reconciles to ready and not active polling", func() {
				rt.Test(rtesting.TableRow{
					Key: key,
					Objects: []runtime.Object{
						sourceResolver,
					},
					WantErr: false,
					WantStatusUpdates: []clientgotesting.UpdateActionImpl{
						{
							Object: &v1alpha1.SourceResolver{
								ObjectMeta: sourceResolver.ObjectMeta,
								Spec:       sourceResolver.Spec,
								Status: v1alpha1.SourceResolverStatus{
									Status: duckv1alpha1.Status{
										ObservedGeneration: originalGeneration,
										Conditions: duckv1alpha1.Conditions{
											{
												Type:   duckv1alpha1.ConditionReady,
												Status: corev1.ConditionTrue,
											},
											{
												Type:   v1alpha1.ActivePolling,
												Status: corev1.ConditionFalse,
											},
										},
									},
									ResolvedSource: v1alpha1.ResolvedSource{
										Blob: &v1alpha1.ResolvedBlobSource{
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
	})
}

func resolvedSourceResolver(sourceResolver *v1alpha1.SourceResolver, resolvedSource v1alpha1.ResolvedSource) *v1alpha1.SourceResolver {
	sourceResolver.ResolvedGitSource(resolvedSource.Git)
	return sourceResolver
}
