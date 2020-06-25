module github.com/pivotal/kpack

go 1.14

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Masterminds/semver/v3 v3.1.0
	github.com/buildpacks/imgutil v0.0.0-20200528211046-5c4cfa56bb24
	github.com/buildpacks/lifecycle v0.8.0
	github.com/docker/docker v17.12.0-ce-rc1.0.20190924003213-a8608b5b67c7+incompatible // indirect
	github.com/go-git/go-git-fixtures v3.5.0+incompatible
	github.com/go-git/go-git/v5 v5.1.0
	github.com/go-openapi/spec v0.19.8
	github.com/google/go-cmp v0.4.1
	github.com/google/go-containerregistry v0.1.0
	github.com/gophercloud/gophercloud v0.4.0 // indirect
	github.com/matthewmcnew/archtest v0.0.0-20191014222827-a111193b50ad
	github.com/pkg/errors v0.9.1
	github.com/sclevine/spec v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.0
	github.com/vdemeester/k8s-pkg-credentialprovider v1.17.4
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20200510223506-06a226fb4e37
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
	gomodules.xyz/jsonpatch/v2 v2.1.0 // indirect
	k8s.io/api v0.17.5
	k8s.io/apiextensions-apiserver v0.17.5 // indirect
	k8s.io/apimachinery v0.17.5
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/code-generator v0.18.4
	k8s.io/kube-openapi v0.0.0-20200316234421-82d701f24f9d
	knative.dev/pkg v0.0.0-20200528142800-1c6815d7e4c9
)

replace (
	k8s.io/client-go => k8s.io/client-go v0.17.5
	k8s.io/code-generator => k8s.io/code-generator v0.17.5
)

exclude github.com/Azure/go-autorest v12.0.0+incompatible
