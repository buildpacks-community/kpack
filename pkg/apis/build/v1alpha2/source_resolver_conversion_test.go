package v1alpha2

import (
	"context"
	"testing"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	corev1alpha1 "github.com/pivotal/kpack/pkg/apis/core/v1alpha1"
	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSourceResolverConverstion(t *testing.T) {
	spec.Run(t, "testClusterStackConversion", testSourceResolverConverstion)
}

func testSourceResolverConverstion(t *testing.T, when spec.G, it spec.S) {
	when("converting to and from v1alpha1", func() {
		v1alpha2SourceResolver := &SourceResolver{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-source-resolver",
				Namespace:  "some-namespace",
				Generation: 0,
			},
			Spec: SourceResolverSpec{
				ServiceAccountName: "some-service-account",
				Source: corev1alpha1.SourceConfig{
					Git: &corev1alpha1.Git{
						URL:      "github.com/my-awesome-project-by-me",
						Revision: "some-revision",
					},
				},
			},
			Status: SourceResolverStatus{
				Status: corev1alpha1.Status{},
				Source: corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      "github.com/my-awesome-project-by-me",
						Revision: "some-revision",
						SubPath:  "some-subpath",
						Type:     "some-type",
					},
				},
			},
		}

		v1alpha1SourceResolver := &v1alpha1.SourceResolver{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-source-resolver",
				Namespace:  "some-namespace",
				Generation: 0,
			},
			Spec: v1alpha1.SourceResolverSpec{
				ServiceAccount: "some-service-account",
				Source: corev1alpha1.SourceConfig{
					Git: &corev1alpha1.Git{
						URL:      "github.com/my-awesome-project-by-me",
						Revision: "some-revision",
					},
				},
			},
			Status: v1alpha1.SourceResolverStatus{
				Status: corev1alpha1.Status{},
				Source: corev1alpha1.ResolvedSourceConfig{
					Git: &corev1alpha1.ResolvedGitSource{
						URL:      "github.com/my-awesome-project-by-me",
						Revision: "some-revision",
						SubPath:  "some-subpath",
						Type:     "some-type",
					},
				},
			},
		}

		it("successfully converts between api versions", func() {

			testV1alpha1SourceResolver := &v1alpha1.SourceResolver{}
			err := v1alpha2SourceResolver.ConvertTo(context.TODO(), testV1alpha1SourceResolver)
			require.NoError(t, err)
			require.Equal(t, v1alpha1SourceResolver, testV1alpha1SourceResolver)

			testV1alpha2SourceResolver := &SourceResolver{}
			err = testV1alpha2SourceResolver.ConvertFrom(context.TODO(), v1alpha1SourceResolver)
			require.NoError(t, err)
			require.Equal(t, testV1alpha2SourceResolver, v1alpha2SourceResolver)
		})
	})
}
