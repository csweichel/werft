module github.com/csweichel/werft

go 1.13

require (
	github.com/GeertJohan/go.rice v1.0.2
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/Microsoft/go-winio v0.4.15-0.20190919025122-fc70bd9a86b5 // indirect
	github.com/alecthomas/repr v0.0.0-20210301060118-828286944d6a
	github.com/buildkite/terminal-to-html v3.2.0+incompatible
	github.com/containerd/containerd v1.4.1 // indirect
	github.com/daaku/go.zipexe v1.0.1 // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/docker/docker v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang-migrate/migrate/v4 v4.11.0
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.2.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/improbable-eng/grpc-web v0.14.0
	github.com/lib/pq v1.10.0
	github.com/mitchellh/copystructure v1.1.1 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/olebedev/emitter v0.0.0-20190110104742-e8d1457e6aee
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/paulbellamy/ratecounter v0.2.0
	github.com/prometheus/client_golang v1.5.1
	github.com/prometheus/common v0.10.0 // indirect
	github.com/prometheus/procfs v0.2.0 // indirect
	github.com/rs/cors v1.7.0 // indirect
	github.com/segmentio/textio v1.2.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.3
	github.com/technosophos/moniker v0.0.0-20210218184952-3ea787d3943b
	golang.org/x/tools v0.1.5
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	google.golang.org/grpc v1.36.1
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	gotest.tools v2.2.0+incompatible // indirect
	k8s.io/api v0.23.1
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.0.0-00010101000000-000000000000
	nhooyr.io/websocket v1.8.6 // indirect
)

replace k8s.io/api => k8s.io/api v0.23.1

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.23.1

replace k8s.io/apimachinery => k8s.io/apimachinery v0.23.1

replace k8s.io/apiserver => k8s.io/apiserver v0.23.1

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.23.1

replace k8s.io/client-go => k8s.io/client-go v0.23.1

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.23.1

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.23.1

replace k8s.io/code-generator => k8s.io/code-generator v0.23.1

replace k8s.io/component-base => k8s.io/component-base v0.23.1

replace k8s.io/cri-api => k8s.io/cri-api v0.23.1

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.23.1

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.23.1

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.23.1

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.23.1

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.23.1

replace k8s.io/kubelet => k8s.io/kubelet v0.23.1

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.23.1

replace k8s.io/metrics => k8s.io/metrics v0.23.1

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.23.1

replace k8s.io/component-helpers => k8s.io/component-helpers v0.23.1

replace k8s.io/controller-manager => k8s.io/controller-manager v0.23.1

replace k8s.io/kubectl => k8s.io/kubectl v0.23.1

replace k8s.io/mount-utils => k8s.io/mount-utils v0.23.1
