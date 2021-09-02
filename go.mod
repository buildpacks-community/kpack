module github.com/pivotal/kpack

go 1.16

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aryann/difflib v0.0.0-20170710044230-e206f873d14a
	github.com/buildpacks/imgutil v0.0.0-20210315155240-52098da06639
	github.com/buildpacks/lifecycle v0.10.2
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.20.3
	github.com/google/go-cmp v0.5.6
	github.com/google/go-containerregistry v0.6.0
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20210322164748-a11b12f378b5
	github.com/jinzhu/gorm v1.9.12 // indirect
	github.com/libgit2/git2go/v31 v31.4.14
	github.com/matthewmcnew/archtest v0.0.0-20191014222827-a111193b50ad
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sabhiram/go-gitignore v0.0.0-20201211210132-54b8a0bf510f
	github.com/sclevine/spec v1.4.0
	github.com/sigstore/cosign v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/theupdateframework/notary v0.6.2-0.20200804143915-84287fd8df4f
	github.com/vdemeester/k8s-pkg-credentialprovider v1.19.7
	go.uber.org/zap v1.18.1
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/code-generator v0.20.7
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	knative.dev/pkg v0.0.0-20210819054404-bda81c029160
)

replace (
	github.com/prometheus/common => github.com/prometheus/common v0.26.0
	k8s.io/client-go => k8s.io/client-go v0.20.7
	k8s.io/api => k8s.io/api v0.20.7
	github.com/tj/assert => github.com/tj/assert v0.0.3
)
