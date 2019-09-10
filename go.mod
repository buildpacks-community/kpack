module github.com/pivotal/kpack

go 1.12

require (
	contrib.go.opencensus.io/exporter/ocagent v0.6.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.0.0-20190211065954-c06c82c832ed // indirect
	github.com/Azure/go-autorest v0.0.0-00010101000000-000000000000 // indirect
	github.com/buildpack/imgutil v0.0.0-20190726132853-1f31ed20483a
	github.com/buildpack/lifecycle v0.3.0
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/google/btree v1.0.0 // indirect
	github.com/google/go-cmp v0.3.0
	github.com/google/go-containerregistry v0.0.0-20190503220729-1c6c7f61e8a5
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/gophercloud/gophercloud v0.4.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/knative/pkg v0.0.0-20190624141606-d82505e6c5b4
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v1.1.0 // indirect
	github.com/sclevine/spec v1.2.0
	github.com/smartystreets/goconvey v0.0.0-20190731233626-505e41936337 // indirect
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/stretchr/testify v1.3.0
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v0.0.0-20180814183419-67bc79d13d15
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/src-d/go-git-fixtures.v3 v3.5.0
	gopkg.in/src-d/go-git.v4 v4.13.1
	k8s.io/api v0.0.0-20190831074750-7364b6bdad65
	k8s.io/apimachinery v0.0.0-20190831074630-461753078381
	k8s.io/client-go v0.0.0-20190831074946-3fe2abece89e
	k8s.io/code-generator v0.0.0-20190831074504-732c9ca86353 // indirect
	k8s.io/kube-openapi v0.0.0-20190816220812-743ec37842bf // indirect
)

replace (
	contrib.go.opencensus.io/exporter/stackdriver => contrib.go.opencensus.io/exporter/stackdriver v0.0.0-20190211065954-c06c82c832ed
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v10.14.0+incompatible
	github.com/census-instrumentation/opencensus-proto => github.com/census-instrumentation/opencensus-proto v0.1.0
	github.com/knative/pkg => github.com/knative/pkg v0.0.0-20190624141606-d82505e6c5b4
	go.opencensus.io => go.opencensus.io v0.20.2
	golang.org/x/net => golang.org/x/net v0.0.0-20190620200207-3b0461eec859
	k8s.io/api => k8s.io/api v0.0.0-20190226173710-145d52631d00
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221084156-01f179d85dbc
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190226174127-78295b709ec6
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20181128191024-b1289fc74931
)
