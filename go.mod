module github.com/pivotal/kpack

go 1.15

require (
	cloud.google.com/go/storage v1.8.0 // indirect
	github.com/Azure/azure-pipeline-go v0.2.2 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.4.2 // indirect
	github.com/BurntSushi/toml v0.3.1
	github.com/Djarvur/go-err113 v0.1.0 // indirect
	github.com/Masterminds/semver/v3 v3.1.0
	github.com/aryann/difflib v0.0.0-20170710044230-e206f873d14a
	github.com/bombsimon/wsl/v2 v2.2.0 // indirect
	github.com/bombsimon/wsl/v3 v3.1.0 // indirect
	github.com/buildpacks/imgutil v0.0.0-20201211223552-8581300fe2b2
	github.com/buildpacks/lifecycle v0.10.2
	github.com/docker/docker v17.12.0-ce-rc1.0.20190924003213-a8608b5b67c7+incompatible // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-critic/go-critic v0.4.3 // indirect
	github.com/go-git/go-git-fixtures v3.5.0+incompatible
	github.com/go-git/go-git/v5 v5.1.0
	github.com/go-openapi/spec v0.19.9
	github.com/go-toolsmith/typep v1.0.2 // indirect
	github.com/golangci/gocyclo v0.0.0-20180528144436-0a533e8fa43d // indirect
	github.com/golangci/misspell v0.3.5 // indirect
	github.com/golangci/revgrep v0.0.0-20180812185044-276a5c0a1039 // indirect
	github.com/google/go-cmp v0.5.4
	github.com/google/go-containerregistry v0.3.0
	github.com/google/wire v0.4.0 // indirect
	github.com/gophercloud/gophercloud v0.4.0 // indirect
	github.com/goreleaser/goreleaser v0.136.0 // indirect
	github.com/goreleaser/nfpm v1.3.0 // indirect
	github.com/gostaticanalysis/analysisutil v0.0.3 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.6 // indirect
	github.com/jirfag/go-printf-func-name v0.0.0-20200119135958-7558a9eaa5af // indirect
	github.com/matthewmcnew/archtest v0.0.0-20191014222827-a111193b50ad
	github.com/mattn/go-colorable v0.1.6 // indirect
	github.com/mattn/go-ieproxy v0.0.1 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b
	github.com/mitchellh/mapstructure v1.3.1 // indirect
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/ryancurrah/gomodguard v1.1.0 // indirect
	github.com/sclevine/spec v1.4.0
	github.com/securego/gosec v0.0.0-20200401082031-e946c8c39989 // indirect
	github.com/sourcegraph/go-diff v0.5.3 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/tdakkota/asciicheck v0.0.0-20200416200610-e657995f937b // indirect
	github.com/tetafro/godot v0.4.2 // indirect
	github.com/theupdateframework/notary v0.6.2-0.20200804143915-84287fd8df4f
	github.com/timakin/bodyclose v0.0.0-20200424151742-cb6215831a94 // indirect
	github.com/vdemeester/k8s-pkg-credentialprovider v1.18.1-0.20201019120933-f1d16962a4db
	github.com/xanzy/go-gitlab v0.32.0 // indirect
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	google.golang.org/api v0.25.0 // indirect
	gopkg.in/ini.v1 v1.56.0 // indirect
	honnef.co/go/tools v0.0.1-2020.1.4 // indirect
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v11.0.1-0.20190805182717-6502b5e7b1b5+incompatible
	k8s.io/code-generator v0.18.6
	k8s.io/kube-openapi v0.0.0-20200410145947-bcb3869e6f29
	knative.dev/pkg v0.0.0-20200702222342-ea4d6e985ba0
	mvdan.cc/unparam v0.0.0-20200501210554-b37ab49443f7 // indirect
	sourcegraph.com/sqs/pbtypes v1.0.0 // indirect
)

replace (
	k8s.io/client-go => k8s.io/client-go v0.17.6
	k8s.io/code-generator => k8s.io/code-generator v0.17.6
)

exclude github.com/Azure/go-autorest v12.0.0+incompatible
