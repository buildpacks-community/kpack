package git

import (
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/apis/build/v1alpha1"
	"github.com/pivotal/build-service-system/pkg/client/clientset/versioned/fake"
	"github.com/pivotal/build-service-system/pkg/client/informers/externalversions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"

	"github.com/sclevine/spec"
)

func TestSourceResolverEnqueuer(t *testing.T) {
	spec.Run(t, "Test SourceResolverEnqueuer", testSourceResolverEnqueuer)
}

func testSourceResolverEnqueuer(t *testing.T, when spec.G, it spec.S) {
	when("#Run", func() {
		fakeClient := fake.NewSimpleClientset()

		informerFactory := externalversions.NewSharedInformerFactory(fakeClient, time.Second)
		sourceResolverInformer := informerFactory.Build().V1alpha1().SourceResolvers()
		pollChan := make(chan string)
		enqueuer := &SourceResolverEnqueuer{
			Frequency:            time.Second,
			PollChan:             pollChan,
			SourceResolverLister: sourceResolverInformer.Lister(),
		}
		stopChan := make(chan struct{})

		informerStopChan := make(chan struct{})
		it.Before(func() {
			informerFactory.Start(informerStopChan)
			informerFactory.WaitForCacheSync(informerStopChan)
		})

		it.After(func() {
			close(informerStopChan)
		})

		it("only process the source resolvers that need polling", func() {
			defer close(stopChan)
			go func() {
				err := enqueuer.Run(stopChan)
				require.NoError(t, err)
			}()

			_, err := fakeClient.BuildV1alpha1().SourceResolvers("some-namespace").
				Create(&v1alpha1.SourceResolver{
					ObjectMeta: v1.ObjectMeta{
						Name: "hasnt-reconciled-yet",
					},
				})
			require.NoError(t, err)

			_, err = fakeClient.BuildV1alpha1().SourceResolvers("some-namespace").
				Create(&v1alpha1.SourceResolver{
					ObjectMeta: v1.ObjectMeta{
						Name: "polling-source",
					},
					Status: v1alpha1.SourceResolverStatus{
						Status: duckv1alpha1.Status{
							Conditions: []duckv1alpha1.Condition{
								{
									Type:   v1alpha1.ActivePolling,
									Status: corev1.ConditionTrue,
								},
							},
						},
					},
				})
			require.NoError(t, err)

			sourceResolverKey := <-pollChan
			assert.Equal(t, "some-namespace/polling-source", sourceResolverKey)
			assert.Empty(t, pollChan)
		})

		it("stops running on closed stopchan", func() {
			go func() {
				close(stopChan)
			}()

			err := enqueuer.Run(stopChan)
			require.NoError(t, err)
		})

		it("stops running on closed stopchan while processing source resolvers", func() {
			go func() {
				_, err := fakeClient.BuildV1alpha1().SourceResolvers("some-namespace").
					Create(&v1alpha1.SourceResolver{
						ObjectMeta: v1.ObjectMeta{
							Name: "polling-source",
						},
						Status: v1alpha1.SourceResolverStatus{
							Status: duckv1alpha1.Status{
								Conditions: []duckv1alpha1.Condition{
									{
										Type:   v1alpha1.ActivePolling,
										Status: corev1.ConditionTrue,
									},
								},
							},
						},
					})
				require.NoError(t, err)

				close(stopChan)
			}()

			err := enqueuer.Run(stopChan)
			require.NoError(t, err)
		})

	})
}
