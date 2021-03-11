module github.com/csweichel/werft/plugins/cron

go 1.13

replace github.com/csweichel/werft => ../..

require (
	github.com/csweichel/werft v0.0.0-00010101000000-000000000000
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.8.1
)
