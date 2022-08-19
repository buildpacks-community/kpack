// Copyright 2018 Google LLC All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sdockercreds

import (
	"context"
	"io/ioutil"
	"os"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
	kauth "github.com/google/go-containerregistry/pkg/authn/kubernetes"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"k8s.io/client-go/kubernetes"

	"github.com/pivotal/kpack/pkg/dockercreds/k8sdockercreds/azurecredentialhelperfix"
)

// Copied from https://github.com/google/go-containerregistry/blob/main/pkg/authn/k8schain/k8schain.go
// This allows us to support AZURE_CONTAINER_REGISTRY_CONFIG

var (
	amazonKeychain authn.Keychain = authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(ioutil.Discard)))
	azureKeychain  authn.Keychain = authn.NewKeychainFromHelper(credhelper.NewACRCredentialsHelper())
)

// Options holds configuration data for guiding credential resolution.
type Options = kauth.Options

// New returns a new authn.Keychain suitable for resolving image references as
// scoped by the provided Options.  It speaks to Kubernetes through the provided
// client interface.
func New(ctx context.Context, client kubernetes.Interface, opt Options) (authn.Keychain, error) {
	if os.Getenv("AZURE_CONTAINER_REGISTRY_CONFIG") != "" {
		azureKeychain = authn.NewMultiKeychain(azurecredentialhelperfix.AzureFileKeychain(), azureKeychain)
	}
	k8s, err := kauth.New(ctx, client, kauth.Options(opt))
	if err != nil {
		return nil, err
	}

	return authn.NewMultiKeychain(
		k8s,
		authn.DefaultKeychain,
		google.Keychain,
		amazonKeychain,
		azureKeychain,
	), nil
}

func NewNoClient(ctx context.Context) (authn.Keychain, error) {
	if os.Getenv("AZURE_CONTAINER_REGISTRY_CONFIG") != "" {
		azureKeychain = authn.NewMultiKeychain(azurecredentialhelperfix.AzureFileKeychain(), azureKeychain)
	}
	return authn.NewMultiKeychain(
		authn.DefaultKeychain,
		google.Keychain,
		amazonKeychain,
		azureKeychain,
	), nil
}
