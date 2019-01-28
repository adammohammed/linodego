module bits.linode.com/asauber/cluster-api-provider-lke

require (
	cloud.google.com/go v0.29.0 // indirect
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/Masterminds/sprig v2.17.1+incompatible // indirect
	github.com/aokoli/goutils v1.1.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/emicklei/go-restful v2.8.1+incompatible // indirect
	github.com/gardener/gardener v0.0.0-20190111065104-f865f232
	github.com/gardener/machine-controller-manager v0.0.0-20190125105937-98eb4668d237 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0 // indirect
	github.com/go-logr/zapr v0.1.0 // indirect
	github.com/go-openapi/spec v0.18.0 // indirect
	github.com/go-resty/resty v1.11.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.1.1 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20180924190550-6f2cf27854a4 // indirect
	github.com/google/btree v0.0.0-20180813153112-4030bb1f1f0c // indirect
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf // indirect
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/hashicorp/golang-lru v0.5.0 // indirect
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/json-iterator/go v1.1.5 // indirect
	github.com/linode/linodego v0.5.1
	github.com/mattbaird/jsonpatch v0.0.0-20171005235357-81af80346b1a // indirect
	github.com/miekg/dns v1.1.3 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742 // indirect
	github.com/onsi/gomega v1.4.2
	github.com/pborman/uuid v1.2.0 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/pkg/errors v0.8.0 // indirect
	github.com/prometheus/client_golang v0.9.2 // indirect
	github.com/sirupsen/logrus v1.3.0 // indirect
	github.com/spf13/pflag v1.0.3 // indirect
	github.com/ugorji/go/codec v0.0.0-20190126102652-8fd0f8d918c8 // indirect
	go.uber.org/atomic v1.3.2 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.9.1 // indirect
	golang.org/x/crypto v0.0.0-20181001203147-e3636079e1a4 // indirect
	golang.org/x/net v0.0.0-20181220203305-927f97764cc3
	golang.org/x/oauth2 v0.0.0-20181003184128-c57b0facaced
	golang.org/x/sys v0.0.0-20181004145325-8469e314837c // indirect
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2 // indirect
	golang.org/x/tools v0.0.0-20190125232054-d66bd3c5d5a6 // indirect
	google.golang.org/appengine v1.2.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/api v0.0.0-20190126160303-6b7c4c78788565e25d
	k8s.io/apiextensions-apiserver v0.0.0-20180808065829-ba848ee89ca33b3 // indirect
	k8s.io/apimachinery v0.0.0-20181127105237-6dd46049f
	k8s.io/client-go v9.0.0+incompatible
	k8s.io/code-generator v0.0.0-20181206115026-3a2206dd6a78 // indirect
	k8s.io/gengo v0.0.0-20190128074634-0689ccc1d7d6 // indirect
	k8s.io/helm v2.12.3+incompatible // indirect
	k8s.io/klog v0.1.0 // indirect
	k8s.io/kube-aggregator v0.0.0-20190119022701-4764f3a1991175f47 // indirect
	k8s.io/kube-openapi v0.0.0-20190115222348-ced9eb3070a5 // indirect
	sigs.k8s.io/cluster-api v0.0.0-20181211193542-3547f8dd9307
	sigs.k8s.io/controller-runtime v0.1.9
	sigs.k8s.io/testing_frameworks v0.1.0 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

replace github.com/go-resty/resty v1.11.0 => gopkg.in/resty.v1 v1.11.0