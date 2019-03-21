module bits.linode.com/asauber/cluster-api-provider-lke

require (
	bits.linode.com/aprotopopov/wg-controller v0.0.0-20190128165615-cef2ac8e0cfce448445457f0465b4821e4a11209
	bits.linode.com/aprotopopov/wg-node-controller v0.0.0-20190115203717-22e8fc3a7005ef44e23771c22a0b1784a4863818
	cloud.google.com/go v0.29.0 // indirect
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/Masterminds/sprig v2.17.1+incompatible // indirect
	github.com/aokoli/goutils v1.1.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/dnaeon/go-vcr v1.0.1 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/gardener/gardener v0.0.0-20190111065104-f865f232
	github.com/gardener/machine-controller-manager v0.0.0-20190125105937-98eb4668d237 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0 // indirect
	github.com/go-logr/zapr v0.1.0 // indirect
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
	github.com/linode/linodego v0.7.0
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
	go.uber.org/atomic v1.3.2 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go.uber.org/zap v1.9.1 // indirect
	golang.org/x/net v0.0.0-20190311183353-d8887717615a
	golang.org/x/oauth2 v0.0.0-20181003184128-c57b0facaced
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2 // indirect
	golang.org/x/tools v0.0.0-20190319232107-3f1ed9edd1b4 // indirect
	google.golang.org/appengine v1.4.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/resty.v1 v1.11.0 // indirect
	k8s.io/api v0.0.0-20190126160303-6b7c4c78788565e25d
	k8s.io/apiextensions-apiserver v0.0.0-20180808065829-ba848ee89ca33b3 // indirect
	k8s.io/apimachinery v0.0.0-20181127105237-6dd46049f
	k8s.io/client-go v9.0.0+incompatible
	k8s.io/code-generator v0.0.0-20190311155051-e4c2b1329cf7 // indirect
	k8s.io/gengo v0.0.0-20190319205223-bc9033e9ec9e // indirect
	k8s.io/helm v2.12.3+incompatible // indirect
	k8s.io/klog v0.1.0 // indirect
	k8s.io/kube-aggregator v0.0.0-20190119022701-4764f3a1991175f47 // indirect
	k8s.io/kube-openapi v0.0.0-20190115222348-ced9eb3070a5 // indirect
	sigs.k8s.io/cluster-api v0.0.0-20181211193542-3547f8dd9307
	sigs.k8s.io/controller-runtime v0.1.9
	sigs.k8s.io/testing_frameworks v0.1.1 // indirect
	sigs.k8s.io/yaml v1.1.0 // indirect
)

replace github.com/go-resty/resty v1.11.0 => gopkg.in/resty.v1 v1.11.0
