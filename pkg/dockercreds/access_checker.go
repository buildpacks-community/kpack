package dockercreds

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/pkg/errors"
)

func VerifyWriteAccess(keychain authn.Keychain, tag string) error {
	var auth authn.Authenticator
	ref, err := name.ParseReference(tag, name.WeakValidation)
	if err != nil {
		return errors.Wrapf(err, "Error parsing reference %q", tag)
	}

	auth, err = keychain.Resolve(ref.Context().Registry)
	if err != nil {
		return errors.Wrap(err, "Error resolving credentials")
	}

	scopes := []string{ref.Scope(transport.PushScope)}
	tr, err := transport.New(ref.Context().Registry, auth, http.DefaultTransport, scopes)
	if err != nil {
		if transportError, ok := err.(*transport.Error); ok {
			for _, diagnosticError := range transportError.Errors {
				if diagnosticError.Code == transport.UnauthorizedErrorCode {
					return errors.Wrap(err, "Unauthorized")
				}
			}

			if transportError.StatusCode == 401 {
				return errors.Wrap(err, "Unauthorized")
			}
		}

		return errors.WithStack(err)
	}

	client := &http.Client{Transport: tr}

	u := url.URL{
		Scheme: ref.Context().Registry.Scheme(),
		Host:   ref.Context().RegistryStr(),
		Path:   fmt.Sprintf("/v2/%s/blobs/uploads/", ref.Context().RepositoryStr()),
	}

	// Make the request to initiate the blob upload.
	resp, err := client.Post(u.String(), "application/json", nil)
	if err != nil {
		return errors.WithStack(err)
	}
	defer resp.Body.Close()

	if err = transport.CheckError(resp, http.StatusCreated, http.StatusAccepted); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func VerifyReadAccess(keychain authn.Keychain, tag string) error {
	ref, err := name.ParseReference(tag, name.WeakValidation)
	if err != nil {
		return errors.Wrapf(err, "Error parsing reference %q", tag)
	}

	if _, err = remote.Get(ref, remote.WithAuthFromKeychain(keychain), remote.WithTransport(http.DefaultTransport)); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
