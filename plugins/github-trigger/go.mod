module github.com/csweichel/werft/plugins/github-trigger

go 1.13

replace github.com/csweichel/werft => ../..

require (
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/csweichel/werft v0.0.0-00010101000000-000000000000
	github.com/google/go-github/v28 v28.1.1 // indirect
	github.com/google/go-github/v31 v31.0.0
	github.com/sirupsen/logrus v1.4.2
	google.golang.org/grpc v1.30.0
	k8s.io/api v0.0.0-20190620084959-7cf5895f2711
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)
