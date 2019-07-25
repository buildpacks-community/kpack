package sourceresolver_test

import (
	"context"
	"testing"
	"time"

	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	knCtrl "github.com/knative/pkg/controller"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/pivotal/build-service-beam/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-beam/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-beam/pkg/client/informers/externalversions"
	"github.com/pivotal/build-service-beam/pkg/git"
	"github.com/pivotal/build-service-beam/pkg/reconciler/testhelpers"
	"github.com/pivotal/build-service-beam/pkg/reconciler/v1alpha1/sourceresolver"
	"github.com/pivotal/build-service-beam/pkg/reconciler/v1alpha1/sourceresolver/sourceresolverfakes"
	"github.com/pivotal/build-service-beam/pkg/secret"
	secrettesthelpers "github.com/pivotal/build-service-beam/pkg/secret/testhelpers"
)

func TestSourceResolver(t *testing.T) {
	spec.Run(t, "Source Resolver Reconciler", testSourceResolver)
}

func testSourceResolver(t *testing.T, when spec.G, it spec.S) {
	fakeClient := fake.NewSimpleClientset()
	callCounter := &testhelpers.CallCounter{}
	fakeClient.PrependReactor("*", "sourceresolvers", callCounter.Reactor)
	k8sFakeClient := k8sfake.NewSimpleClientset()

	fakeGitResolver := &sourceresolverfakes.FakeGitResolver{}
	fakeEnqueuer := &sourceresolverfakes.FakeEnqueuer{}

	reconciler := testhelpers.RebuildingReconciler(func() knCtrl.Reconciler {
		informerFactory := externalversions.NewSharedInformerFactory(fakeClient, time.Second)
		sourceResolverInformer := informerFactory.Build().V1alpha1().SourceResolvers()

		return testhelpers.SyncWaitingReconciler(
			informerFactory,
			&sourceresolver.Reconciler{
				GitKeychain:          git.NewK8sGitKeychain(k8sFakeClient),
				Client:               fakeClient,
				GitResolver:          fakeGitResolver,
				SourceResolverLister: sourceResolverInformer.Lister(),
				Enqueuer:             fakeEnqueuer,
			},
			sourceResolverInformer.Informer().HasSynced,
		)
	})

	const (
		sourceResolverName       = "source-resolver-name"
		namespace                = "some-namespace"
		key                      = "some-namespace/source-resolver-name"
		originalGeneration int64 = 1
		serviceAccount           = "serviceAccount"
	)

	sourceResolver := &v1alpha1.SourceResolver{
		ObjectMeta: v1.ObjectMeta{
			Name:       sourceResolverName,
			Namespace:  namespace,
			Generation: originalGeneration,
		},
		Spec: v1alpha1.SourceResolverSpec{
			ServiceAccount: serviceAccount,
			Source: v1alpha1.Source{
				Git: v1alpha1.Git{
					URL:      "https://github.com/build-me",
					Revision: "1234",
				},
			},
		},
	}

	when("#Reconcile", func() {
		it.Before(func() {
			_, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).Create(sourceResolver)
			require.NoError(t, err)
		})

		it("updates the observed generation", func() {
			err := reconciler.Reconcile(context.TODO(), key)
			require.NoError(t, err)

			resolver, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).Get(sourceResolverName, v1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, resolver.Status.ObservedGeneration, originalGeneration)
		})

		it("requests resolved git with the source config and the auth", func() {
			err := secrettesthelpers.SaveGitSecrets(k8sFakeClient, namespace, serviceAccount, []secret.URLAndUser{
				{
					URL:      "github.com",
					Username: "some-username",
					Password: "some-password",
				},
			})
			require.NoError(t, err)

			err = reconciler.Reconcile(context.TODO(), key)
			require.NoError(t, err)

			require.Equal(t, fakeGitResolver.ResolveCallCount(), 1)
			auth, sourceArg := fakeGitResolver.ResolveArgsForCall(0)
			require.Equal(t, auth, git.BasicAuth{
				Username: "some-username",
				Password: "some-password",
			})
			require.Equal(t, sourceArg, v1alpha1.Git{
				URL:      "https://github.com/build-me",
				Revision: "1234",
			})
		})

		it("does not unnecessarily update the resource", func() {
			err := reconciler.Reconcile(context.TODO(), key)
			require.NoError(t, err)
			err = reconciler.Reconcile(context.TODO(), key)
			require.NoError(t, err)
			err = reconciler.Reconcile(context.TODO(), key)
			require.NoError(t, err)

			require.Equal(t, callCounter.UpdateCalls(), 1)
		})

		when("a specific commit sha is the source", func() {
			it.Before(func() {
				fakeGitResolver.ResolveReturns(v1alpha1.ResolvedGitSource{
					URL:      "https://github.com/build-me",
					Revision: "1234",
					Type:     v1alpha1.Commit,
				}, nil)

			})

			it("reconciles to ready and not active polling", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				resolver, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).Get(sourceResolverName, v1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, resolver.Status.ResolvedSource.Git, v1alpha1.ResolvedGitSource{
					URL:      "https://github.com/build-me",
					Revision: "1234",
					Type:     v1alpha1.Commit,
				})
				assert.True(t, resolver.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue())
				assert.True(t, resolver.Status.GetCondition(v1alpha1.ActivePolling).IsFalse())
			})

			it("does not enqueue subsequent processing", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				require.Equal(t, 0, fakeEnqueuer.EnqueueCallCount())
			})
		})

		when("a branch is the source", func() {
			it("reconciles to ready and active polling", func() {
				fakeGitResolver.ResolveReturns(v1alpha1.ResolvedGitSource{
					URL:      "https://github.com/build-me",
					Revision: "1234",
					Type:     v1alpha1.Branch,
				}, nil)

				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				resolver, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).Get(sourceResolverName, v1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, resolver.Status.ResolvedSource.Git, v1alpha1.ResolvedGitSource{
					URL:      "https://github.com/build-me",
					Revision: "1234",
					Type:     v1alpha1.Branch,
				})
				assert.True(t, resolver.Status.GetCondition(duckv1alpha1.ConditionReady).IsTrue())
				assert.True(t, resolver.Status.GetCondition(v1alpha1.ActivePolling).IsTrue())
			})

			it("enqueues source resolvers for subsequent processing", func() {
				fakeGitResolver.ResolveReturns(v1alpha1.ResolvedGitSource{
					URL:      "https://github.com/build-me",
					Revision: "1234",
					Type:     v1alpha1.Branch,
				}, nil)

				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				require.Equal(t, 1, fakeEnqueuer.EnqueueCallCount())
				resolver, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).Get(sourceResolverName, v1.GetOptions{})
				require.NoError(t, err)

				require.Equal(t, resolver, fakeEnqueuer.EnqueueArgsForCall(0))
			})
		})

		when("git resolves to unknown", func() {
			it.Before(func() {
				fakeGitResolver.ResolveReturns(v1alpha1.ResolvedGitSource{
					URL:      "https://github.com/build-me",
					Revision: "flaky-branch",
					Type:     v1alpha1.Unknown,
				}, nil)
			})

			it("saves unknown when source has not previously resolved", func() {
				err := reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				resolver, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).Get(sourceResolverName, v1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, resolver.Status.ResolvedSource.Git, v1alpha1.ResolvedGitSource{
					URL:      "https://github.com/build-me",
					Revision: "flaky-branch",
					Type:     v1alpha1.Unknown,
				})
			})

			it("ignores unknown when source has been previously resolved", func() {
				alreadyResolvedSourceResolver := sourceResolver.DeepCopy()
				alreadyResolvedSourceResolver.Status.ObservedGeneration = originalGeneration
				alreadyResolvedSourceResolver.ResolvedGitSource(v1alpha1.ResolvedGitSource{
					URL:      "https://github.com/build-me",
					Revision: "real-commit",
					Type:     v1alpha1.Branch,
				})
				_, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).UpdateStatus(alreadyResolvedSourceResolver)
				require.NoError(t, err)

				err = reconciler.Reconcile(context.TODO(), key)
				require.NoError(t, err)

				resolver, err := fakeClient.BuildV1alpha1().SourceResolvers(namespace).Get(sourceResolverName, v1.GetOptions{})
				require.NoError(t, err)

				assert.Equal(t, v1alpha1.ResolvedGitSource{
					URL:      "https://github.com/build-me",
					Revision: "real-commit",
					Type:     v1alpha1.Branch,
				}, resolver.Status.ResolvedSource.Git)
			})
		})
	})
}
