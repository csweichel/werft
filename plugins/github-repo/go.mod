module github.com/csweichel/werft/plugins/github-repo

go 1.14

replace github.com/csweichel/werft => ../..

require (
	github.com/bradleyfalzon/ghinstallation v1.1.1
	github.com/csweichel/werft v0.0.0-00010101000000-000000000000
	github.com/google/go-cmp v0.5.0
	github.com/google/go-github/v31 v31.0.0
	github.com/sirupsen/logrus v1.4.2
	google.golang.org/grpc v1.30.0
	k8s.io/api v0.0.0-20190620084959-7cf5895f2711
)
