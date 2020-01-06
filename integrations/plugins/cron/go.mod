module github.com/32leaves/werft/cron-plugin

go 1.13

replace github.com/32leaves/werft => ../../..

require (
	github.com/32leaves/werft v0.0.0-00010101000000-000000000000
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.4.2
)
