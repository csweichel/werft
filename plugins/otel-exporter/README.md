This plugin emits OpenTelemetry tracing data for werft builds.

## Configuration
```YAML
# which OTel exporter to use. Supported values are "stdout" and "http"
exporter: "http"
```

When using the `http` exporter, you can configure its behaviour using the `OTEL` environment variables, e.g.
```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://api.honeycomb.io/"
export OTEL_EXPORTER_OTLP_HEADERS="x-honeycomb-team=your-api-key,x-honeycomb-dataset=your-dataset"
export OTEL_SERVICE_NAME="your-service-name"
```
