module bits.linode.com/LinodeAPI/cluster-api-provider-lke

require (
	bits.linode.com/LinodeAPI/wg-controller v0.0.0-20190612172757-4e4c41c20655
	bits.linode.com/LinodeAPI/wg-node-controller v0.0.0-20190612173038-0026e88e77ea
	github.com/Masterminds/sprig v2.17.1+incompatible // indirect
	github.com/aokoli/goutils v1.1.0 // indirect
	github.com/aws/aws-sdk-go v1.22.0
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/dnaeon/go-vcr v1.0.1 // indirect
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20181111060418-2ce16c963a8a // indirect
	github.com/gardener/gardener v0.0.0-20190111065104-f865f232
	github.com/gardener/machine-controller-manager v0.0.0-20190125105937-98eb4668d237 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.2.2-0.20190723190241-65acae22fc9d // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/json-iterator/go v1.1.7 // indirect
	github.com/linode/linodego v0.10.0
	github.com/miekg/dns v1.1.3 // indirect
	github.com/onsi/gomega v1.5.0
	golang.org/x/crypto v0.0.0-20190611184440-5c40567a22f8 // indirect
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc
	golang.org/x/sync v0.0.0-20190423024810-112230192c58 // indirect
	gopkg.in/resty.v1 v1.11.0 // indirect
	k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apimachinery v0.0.0-20190703205208-4cfb76a8bf76
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/helm v2.12.3+incompatible // indirect
	k8s.io/klog v0.4.0 // indirect
	k8s.io/kube-aggregator v0.0.0-20190119022701-4764f3a1991175f47 // indirect
	k8s.io/kube-openapi v0.0.0-20190709113604-33be087ad058 // indirect
	sigs.k8s.io/cluster-api v0.1.10-0.20190821205433-c80f6e5ecc7f
	sigs.k8s.io/controller-runtime v0.1.12
)

replace github.com/go-resty/resty v1.11.0 => gopkg.in/resty.v1 v1.11.0

replace k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
