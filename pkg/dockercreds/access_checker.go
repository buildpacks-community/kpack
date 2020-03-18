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

func HasWriteAccess(keychain authn.Keychain, tag string) (bool, error) {
	var auth authn.Authenticator
	ref, err := name.ParseReference(tag, name.WeakValidation)
	if err != nil {
		return false, err
	}

	auth, err = keychain.Resolve(ref.Context().Registry)
	if err != nil {
		return false, err
	}

	scopes := []string{ref.Scope(transport.PushScope)}
	tr, err := transport.New(ref.Context().Registry, auth, http.DefaultTransport, scopes)
	if err != nil {
		if transportError, ok := err.(*transport.Error); ok {
			for _, diagnosticError := range transportError.Errors {
				if diagnosticError.Code == transport.UnauthorizedErrorCode {
					return false, nil
				}
			}

			if transportError.StatusCode == 401 {
				return false, nil
			}
		}

		return false, errors.WithStack(err)
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
		return false, errors.WithStack(err)
	}
	defer resp.Body.Close()

	if err := transport.CheckError(resp, http.StatusCreated, http.StatusAccepted); err != nil {
		return false, nil
	}

	return true, nil
}

func HasReadAccess(keychain authn.Keychain, tag string) (bool, error) {
	ref, err := name.ParseReference(tag, name.WeakValidation)
	if err != nil {
		return false, errors.Wrapf(err, "parse reference '%s'", tag)
	}
	_, err = remote.Get(ref, remote.WithAuthFromKeychain(keychain), remote.WithTransport(http.DefaultTransport))
	if err != nil {
		if _, ok := err.(*transport.Error); ok {
			return false, nil
		}

		return false, errors.Wrapf(err, "validating read access to: %s", tag)
	}

	return true, nil
}
