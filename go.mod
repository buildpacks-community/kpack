module github.com/pivotal/kpack

go 1.16

require (
	cloud.google.com/go/kms v1.2.0 // indirect
	github.com/BurntSushi/toml v1.0.0
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/aryann/difflib v0.0.0-20170710044230-e206f873d14a
	github.com/buildpacks/imgutil v0.0.0-20220310160537-4dd8bc60eaff
	github.com/buildpacks/lifecycle v0.13.4
	github.com/containerd/containerd v1.6.1 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.11.2 // indirect
	github.com/docker/cli v20.10.13+incompatible // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.13+incompatible // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/spec v0.20.3
	github.com/google/go-cmp v0.5.7
	github.com/google/go-containerregistry v0.8.0
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20210610160139-c086c7f16d4e
	github.com/jinzhu/gorm v1.9.12 // indirect
	github.com/klauspost/compress v1.15.1 // indirect
	github.com/libgit2/git2go/v33 v33.0.4
	github.com/matthewmcnew/archtest v0.0.0-20191014222827-a111193b50ad
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b
	github.com/pkg/errors v0.9.1
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06
	github.com/sclevine/spec v1.4.0
	github.com/sigstore/cosign v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/theupdateframework/notary v0.6.2-0.20200804143915-84287fd8df4f
	github.com/vdemeester/k8s-pkg-credentialprovider v1.20.7
	github.com/whilp/git-urls v1.0.0
	go.uber.org/goleak v1.1.12 // indirect
	go.uber.org/zap v1.19.1
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220310020820-b874c991c1a5 // indirect
	gonum.org/v1/netlib v0.0.0-20190331212654-76723241ea4e // indirect
	google.golang.org/genproto v0.0.0-20220310185008-1973136f34c6 // indirect
	google.golang.org/grpc v1.45.0 // indirect
	k8s.io/api v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/code-generator v0.20.11
	k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7
	knative.dev/pkg v0.0.0-20210902173607-844a6bc45596
	sigs.k8s.io/structured-merge-diff v0.0.0-20190525122527-15d366b2352e // indirect
	sigs.k8s.io/structured-merge-diff/v3 v3.0.0 // indirect
)

replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.5.10
	github.com/prometheus/common => github.com/prometheus/common v0.26.0
	k8s.io/api => k8s.io/api v0.20.11
	k8s.io/client-go => k8s.io/client-go v0.20.11
)
