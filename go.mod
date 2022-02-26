module github.com/pivotal/kpack

go 1.16

require (
	cloud.google.com/go/kms v1.2.0 // indirect
	github.com/BurntSushi/toml v0.4.1
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/aryann/difflib v0.0.0-20170710044230-e206f873d14a
	github.com/buildpacks/imgutil v0.0.0-20210818180451-66aea982d5dc
	github.com/buildpacks/lifecycle v0.10.2
	github.com/containerd/containerd v1.5.9 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.20.4
	github.com/google/go-cmp v0.5.7
	github.com/google/go-containerregistry v0.8.1-0.20220125170349-50dfc2733d10
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20220125170349-50dfc2733d10
	github.com/jinzhu/gorm v1.9.12 // indirect
	github.com/klauspost/compress v1.14.2 // indirect
	github.com/libgit2/git2go/v33 v33.0.4
	github.com/matthewmcnew/archtest v0.0.0-20191014222827-a111193b50ad
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b
	github.com/pkg/errors v0.9.1
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06
	github.com/sclevine/spec v1.4.0
	github.com/sigstore/cosign v1.5.2
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/theupdateframework/notary v0.6.2-0.20200804143915-84287fd8df4f
	github.com/vdemeester/k8s-pkg-credentialprovider v1.20.7
	github.com/whilp/git-urls v1.0.0
	go.uber.org/zap v1.20.0
	golang.org/x/crypto v0.0.0-20220112180741-5e0467b6c7ce
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.44.0 // indirect
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/code-generator v0.22.5
	k8s.io/kube-openapi v0.0.0-20220124234850-424119656bbf
	knative.dev/pkg v0.0.0-20220121092305-3ba5d72e310a
)

replace (
	github.com/prometheus/common => github.com/prometheus/common v0.26.0
	k8s.io/api => k8s.io/api v0.20.11
	k8s.io/client-go => k8s.io/client-go v0.20.11
)
