package registry

import (
	"fmt"
	"net/http"
	"net/url"

	lcAuth "github.com/buildpack/lifecycle/image/auth"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/pkg/errors"
)

func HasWriteAccess(tagName string) (bool, error) {
	keychain := authn.DefaultKeychain

	ref, auth, err := lcAuth.ReferenceForRepoName(keychain, tagName)
	if err != nil {
		return false, errors.WithStack(err)
	}

	recordingTransport := &unAuthorizedWithoutErrorCodeTransportChecker{}

	scopes := []string{ref.Scope(transport.PushScope)}
	tr, err := transport.New(ref.Context().Registry, auth, recordingTransport, scopes)
	if err != nil {
		if transportError, ok := err.(*transport.Error); ok {
			for _, diagnosticError := range transportError.Errors {
				if diagnosticError.Code == transport.UnauthorizedErrorCode {
					return false, nil
				}
			}

			if recordingTransport.wasRequestUnauthorized() {
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

type unAuthorizedWithoutErrorCodeTransportChecker struct {
	isToken401 bool
}

func (h *unAuthorizedWithoutErrorCodeTransportChecker) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := http.DefaultTransport.RoundTrip(r)

	if _, isTokenFetchRequest := r.Header["Authorization"]; isTokenFetchRequest && response != nil {
		h.isToken401 = response.StatusCode == 401
	}

	return response, err
}

func (h *unAuthorizedWithoutErrorCodeTransportChecker) wasRequestUnauthorized() bool {
	return h.isToken401
}
