package git

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"

	"github.com/pivotal/kpack/pkg/apis/build/v1alpha1"
	"github.com/pivotal/kpack/pkg/secret"
	"github.com/pivotal/kpack/pkg/secret/testhelpers"
)

func Test(t *testing.T) {
	spec.Run(t, "Test Git Keychain", test)
}

func test(t *testing.T, when spec.G, it spec.S) {
	const serviceAccount = "some-service-account"

	var (
		fakeClient = fake.NewSimpleClientset()
		keychain   = newK8sGitKeychain(fakeClient)
	)

	it.Before(func() {
		fakeClient.PrependReactor("get", "*", func(action k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
			getAction, ok := action.(k8sTesting.GetAction)
			require.True(t, ok)
			require.NotEqual(t, getAction.GetName(), "", "name must be a valid resource name")

			return false, nil, nil
		})

		err := testhelpers.SaveGitSecrets(fakeClient, "some-namespace", serviceAccount, []secret.URLAndUser{
			{
				URL:      "https://github.com",
				Username: "saved-username",
				Password: "saved-password",
			},
			{
				URL:      "noschemegit.com",
				Username: "noschemegit-username",
				Password: "noschemegit-password",
			},
		})
		require.NoError(t, err)
	})

	when("Resolve", func() {
		it("returns git auth for matching secrets", func() {
			auth, err := keychain.Resolve("some-namespace", serviceAccount, v1alpha1.Git{
				URL:      "https://github.com/org/repo",
				Revision: "master",
			})
			require.NoError(t, err)

			require.Equal(t, auth, basicAuth{
				Username: "saved-username",
				Password: "saved-password",
			})
		})

		it("returns git auth for matching secrets without scheme", func() {
			auth, err := keychain.Resolve("some-namespace", serviceAccount, v1alpha1.Git{
				URL:      "https://noschemegit.com/org/repo",
				Revision: "master",
			})
			require.NoError(t, err)

			require.Equal(t, auth, basicAuth{
				Username: "noschemegit-username",
				Password: "noschemegit-password",
			})
		})

		it("returns anonymous auth for no matching secret", func() {
			auth, err := keychain.Resolve("some-namespace", serviceAccount, v1alpha1.Git{
				URL:      "https://no-creds-github.com/org/repo",
				Revision: "master",
			})
			require.NoError(t, err)

			require.Equal(t, auth, anonymousAuth{})
		})

		it("returns anonymous auth for an empty service account", func() {
			auth, err := keychain.Resolve("some-namespace", "", v1alpha1.Git{
				URL:      "https://no-creds-github.com/org/repo",
				Revision: "master",
			})
			require.NoError(t, err)

			require.Equal(t, auth, anonymousAuth{})
		})
	})
}
