module github.com/csweichel/werft

go 1.13

require (
	github.com/GeertJohan/go.rice v1.0.2
	github.com/Masterminds/sprig/v3 v3.2.2
	github.com/alecthomas/repr v0.0.0-20210301060118-828286944d6a
	github.com/buildkite/terminal-to-html v3.2.0+incompatible
	github.com/daaku/go.zipexe v1.0.1 // indirect
	github.com/desertbit/timer v0.0.0-20180107155436-c41aec40b27f // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/golang/protobuf v1.4.3
	github.com/google/uuid v1.2.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/improbable-eng/grpc-web v0.14.0
	github.com/lib/pq v1.10.0
	github.com/mitchellh/copystructure v1.1.1 // indirect
	github.com/olebedev/emitter v0.0.0-20190110104742-e8d1457e6aee
	github.com/paulbellamy/ratecounter v0.2.0
	github.com/prometheus/client_golang v1.9.0
	github.com/rs/cors v1.7.0 // indirect
	github.com/segmentio/textio v1.2.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.1.3
	github.com/technosophos/moniker v0.0.0-20210218184952-3ea787d3943b
	golang.org/x/crypto v0.0.0-20210220033148-5ea612d1eb83 // indirect
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110 // indirect
	golang.org/x/sys v0.0.0-20210309074719-68d13333faf2 // indirect
	golang.org/x/text v0.3.5 // indirect
	golang.org/x/tools v0.1.0
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1
	google.golang.org/genproto v0.0.0-20210310155132-4ce2db91004e // indirect
	google.golang.org/grpc v1.36.0
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	nhooyr.io/websocket v1.8.6 // indirect
)

replace k8s.io/api => k8s.io/api v0.20.4

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.4

replace k8s.io/apimachinery => k8s.io/apimachinery v0.20.4

replace k8s.io/apiserver => k8s.io/apiserver v0.20.4

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.20.4

replace k8s.io/client-go => k8s.io/client-go v0.20.4

replace k8s.io/cloud-provider => k8s.io/cloud-provider v0.20.4

replace k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.20.4

replace k8s.io/code-generator => k8s.io/code-generator v0.20.4

replace k8s.io/component-base => k8s.io/component-base v0.20.4

replace k8s.io/cri-api => k8s.io/cri-api v0.20.4

replace k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.20.4

replace k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.20.4

replace k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.20.4

replace k8s.io/kube-proxy => k8s.io/kube-proxy v0.20.4

replace k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.20.4

replace k8s.io/kubelet => k8s.io/kubelet v0.20.4

replace k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.20.4

replace k8s.io/metrics => k8s.io/metrics v0.20.4

replace k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.20.4

replace k8s.io/component-helpers => k8s.io/component-helpers v0.20.4

replace k8s.io/controller-manager => k8s.io/controller-manager v0.20.4

replace k8s.io/kubectl => k8s.io/kubectl v0.20.4

replace k8s.io/mount-utils => k8s.io/mount-utils v0.20.4
