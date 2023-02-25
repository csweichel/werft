module github.com/csweichel/werft/plugins/github-integration

go 1.17

require (
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/csweichel/werft v0.1.5
	github.com/csweichel/werft/plugins/github-repo v0.0.0-00010101000000-000000000000
	github.com/golang/mock v1.5.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.7
	github.com/google/go-github/v35 v35.3.0
	github.com/sirupsen/logrus v1.8.1
)

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-github/v29 v29.0.3 // indirect
	github.com/google/go-github/v31 v31.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	golang.org/x/crypto v0.0.0-20220214200702-86341886e292 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20220218161850-94dd64e39d7c // indirect
	google.golang.org/grpc v1.45.0 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	k8s.io/api v0.23.4 // indirect
	k8s.io/apimachinery v0.23.4 // indirect
	k8s.io/klog/v2 v2.30.0 // indirect
	k8s.io/utils v0.0.0-20211116205334-6203023598ed // indirect
	sigs.k8s.io/json v0.0.0-20211020170558-c049b76a60c6 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
)

replace github.com/csweichel/werft => ../.. // leeway

replace github.com/csweichel/werft/plugins/github-repo => ../github-repo // leeway

replace k8s.io/api => k8s.io/api v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/apimachinery => k8s.io/apimachinery v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/apiserver => k8s.io/apiserver v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/client-go => k8s.io/client-go v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/code-generator => k8s.io/code-generator v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/component-base => k8s.io/component-base v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/cri-api => k8s.io/cri-api v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/kubelet => k8s.io/kubelet v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/metrics => k8s.io/metrics v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/component-helpers => k8s.io/component-helpers v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/controller-manager => k8s.io/controller-manager v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/kubectl => k8s.io/kubectl v0.23.4 // leeway indirect from //:plugin-client-lib

replace k8s.io/mount-utils => k8s.io/mount-utils v0.23.4 // leeway indirect from //:plugin-client-lib
