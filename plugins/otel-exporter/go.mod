module github.com/csweichel/werft/plugins/otel-exporter

go 1.17

require (
	github.com/csweichel/werft v0.0.0-00010101000000-000000000000
	github.com/golang/mock v1.5.0
	github.com/sirupsen/logrus v1.8.1
	go.opentelemetry.io/otel v1.7.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.7.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.7.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.7.0
	go.opentelemetry.io/otel/sdk v1.7.0
	go.opentelemetry.io/otel/trace v1.7.0
	google.golang.org/protobuf v1.28.0
)

require (
	github.com/cenkalti/backoff/v4 v4.1.3 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.7.0 // indirect
	go.opentelemetry.io/proto/otlp v0.16.0 // indirect
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20211208223120-3a66f561d7aa // indirect
	google.golang.org/grpc v1.46.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/csweichel/werft => ../.. // leeway

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
