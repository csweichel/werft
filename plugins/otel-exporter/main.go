package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"
	"time"

	v1 "github.com/csweichel/werft/pkg/api/v1"
	"github.com/csweichel/werft/pkg/filterexpr"
	"github.com/csweichel/werft/pkg/plugin/client"
	plugin "github.com/csweichel/werft/pkg/plugin/client"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	// "go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"

	// "go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config configures this plugin
type Config struct {
	Filter   []string     `yaml:"filter"`
	Exporter OTelExporter `yaml:"exporter"`
}

type OTelExporter string

const (
	OTelExporterStdout OTelExporter = "stdout"
	OTelExporterHTTP   OTelExporter = "http"
)

func main() {
	plugin.Serve(&Config{},
		plugin.WithIntegrationPlugin(&otelExporterPlugin{}),
	)
}

type otelExporterPlugin struct{}

func (*otelExporterPlugin) Run(ctx context.Context, config interface{}, srv *client.Services) error {
	cfg, ok := config.(*Config)
	if !ok {
		return fmt.Errorf("config has wrong type %s", reflect.TypeOf(config))
	}

	var opts []sdktrace.TracerProviderOption
	switch cfg.Exporter {
	case OTelExporterStdout:
		out, _ := stdouttrace.New(stdouttrace.WithPrettyPrint())
		opts = append(opts, sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(out)))
	case OTelExporterHTTP:
		exporter, err := otlptrace.New(ctx, otlptracehttp.NewClient())
		if err != nil {
			return err
		}
		opts = append(opts, sdktrace.WithBatcher(exporter))
	default:
		return fmt.Errorf("unsupported exporter: %s", cfg.Exporter)
	}

	tp := sdktrace.NewTracerProvider(opts...)
	defer tp.Shutdown(ctx)
	otel.SetTracerProvider(tp)

	filter, err := filterexpr.Parse(cfg.Filter)
	if err != nil {
		return fmt.Errorf("cannot parse filter: %w", err)
	}

	sub, err := srv.Subscribe(ctx, &v1.SubscribeRequest{
		Filter: []*v1.FilterExpression{{Terms: filter}},
	})
	if err != nil {
		return fmt.Errorf("cannot subscribe: %w", err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	jobs := make(map[string]context.CancelFunc)
	for {
		resp, err := sub.Recv()
		if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || resp == nil {
			return nil
		}
		if err != nil {
			return fmt.Errorf("subscription error: %w", err)
		}

		job := resp.Result
		if _, exists := jobs[job.Name]; exists {
			continue
		}
		if job.Phase == v1.JobPhase_PHASE_DONE || job.Phase == v1.JobPhase_PHASE_CLEANUP {
			continue
		}

		jctx, cancel := context.WithCancel(context.Background())
		jobs[job.Name] = cancel

		wg.Add(1)
		go watchJob(jctx, &wg, srv, job)
	}
}

func watchJob(ctx context.Context, wg *sync.WaitGroup, srv *client.Services, job *v1.JobStatus) {
	defer wg.Done()

	jobName := job.Name
	log := logrus.WithField("job", jobName)

	tracer := otel.GetTracerProvider().Tracer("github.com/csweichel/werft/plugins/otel-exporter")

	log.Info("exporting telemetry for this job")

	var jobAttributes []attribute.KeyValue
	for _, a := range job.Metadata.Annotations {
		jobAttributes = append(jobAttributes, attribute.String(fmt.Sprintf("werft.annotation.%s", a.Key), a.Value))
	}
	jobAttributes = append(jobAttributes, attribute.String("werft.metadata.owner", job.Metadata.Owner))
	jobAttributes = append(jobAttributes, attribute.String("werft.metadata.jobSpecName", job.Metadata.JobSpecName))
	jobAttributes = append(jobAttributes, attribute.String("werft.metadata.trigger", job.Metadata.Trigger.String()))
	jobAttributes = append(jobAttributes, attribute.String("werft.metadata.created", job.Metadata.Created.AsTime().Format(time.RFC3339)))
	if job.Metadata.Repository != nil {
		jobAttributes = append(jobAttributes, attribute.String("werft.metadata.repo.host", job.Metadata.Repository.Host))
		jobAttributes = append(jobAttributes, attribute.String("werft.metadata.repo.owner", job.Metadata.Repository.Owner))
		jobAttributes = append(jobAttributes, attribute.String("werft.metadata.repo.ref", job.Metadata.Repository.Ref))
		jobAttributes = append(jobAttributes, attribute.String("werft.metadata.repo.repo", job.Metadata.Repository.Repo))
		jobAttributes = append(jobAttributes, attribute.String("werft.metadata.repo.revision", job.Metadata.Repository.Revision))
	}

	ctx, jobSpan := tracer.Start(ctx, jobName, trace.WithAttributes(
		attribute.String("werft.type", "job"),
	), trace.WithAttributes(jobAttributes...))
	defer jobSpan.End()

	sub, err := srv.Listen(ctx, &v1.ListenRequest{
		Name:    jobName,
		Updates: true,
		Logs:    v1.ListenRequestLogs_LOGS_RAW,
	})
	if err != nil {
		log.WithError(err).Error("failed to listen to job")
		return
	}
	defer log.Debug("done exporting telemetry for this job")

	var (
		jobPhase     v1.JobPhase
		phaseSpan    trace.Span
		phaseSpanCtx context.Context
		sliceSpans   = make(map[string]trace.Span)
	)
	defer func() {
		if phaseSpan != nil {
			phaseSpan.End()
		}
	}()

	newPhaseSpan := func(name string) {
		if phaseSpan != nil {
			phaseSpan.End()
		}
		phaseSpanCtx, phaseSpan = tracer.Start(ctx, name, trace.WithAttributes(
			attribute.String("werft.type", "phase"),
		), trace.WithAttributes(jobAttributes...))
	}
	handleSlice := func(slice *v1.LogSliceEvent) {
		if slice == nil {
			return
		}

		name := slice.Name

		switch slice.Type {
		case v1.LogSliceType_SLICE_START, v1.LogSliceType_SLICE_CONTENT:
			if phaseSpanCtx == nil {
				// phase hasn't started yet - create a default one
				newPhaseSpan(name)
			}

			if _, ok := sliceSpans[name]; !ok {
				_, s := tracer.Start(phaseSpanCtx, name, trace.WithAttributes(
					attribute.String("werft.type", "slice"),
				), trace.WithAttributes(jobAttributes...))
				sliceSpans[name] = s
			}

		case v1.LogSliceType_SLICE_DONE, v1.LogSliceType_SLICE_ABANDONED, v1.LogSliceType_SLICE_FAIL:
			if s, ok := sliceSpans[name]; ok {
				if slice.Type == v1.LogSliceType_SLICE_FAIL {
					s.SetStatus(codes.Error, slice.Payload)
				}

				s.End()
				delete(sliceSpans, name)
			}

		case v1.LogSliceType_SLICE_PHASE:
			newPhaseSpan(name)

		case v1.LogSliceType_SLICE_RESULT:
			jobSpan.AddEvent("result "+name, trace.WithAttributes(attribute.String("payload", slice.Payload)))
		}
	}

	for {
		update, err := sub.Recv()
		if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) || update == nil {
			return
		}

		switch ctnt := update.Content.(type) {
		case *v1.ListenResponse_Slice:
			handleSlice(ctnt.Slice)
		case *v1.ListenResponse_Update:
			if jobPhase != ctnt.Update.Phase {
				jobSpan.AddEvent(fmt.Sprintf("job-phase-%s", ctnt.Update.Phase))
				jobPhase = ctnt.Update.Phase
			}
			if jobPhase == v1.JobPhase_PHASE_DONE || jobPhase == v1.JobPhase_PHASE_CLEANUP {
				return
			}
		}
	}
}
