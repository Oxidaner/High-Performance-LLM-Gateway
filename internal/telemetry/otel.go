package telemetry

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"llm-gateway/internal/config"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init sets up OpenTelemetry global tracer provider.
func Init(cfg config.MonitoringConfig) (func(context.Context) error, error) {
	if !cfg.OTLP.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	endpoint := strings.TrimSpace(cfg.OTLP.Endpoint)
	if endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	exporterOpts := []otlptracehttp.Option{
		otlptracehttp.WithTimeout(5 * time.Second),
	}

	// Support both "host:port" and full URL formats in config.
	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("invalid otlp endpoint %q: %w", endpoint, err)
		}
		if strings.TrimSpace(parsed.Host) == "" {
			return nil, fmt.Errorf("invalid otlp endpoint %q: host is empty", endpoint)
		}

		exporterOpts = append(exporterOpts, otlptracehttp.WithEndpoint(parsed.Host))
		if path := strings.TrimSpace(parsed.EscapedPath()); path != "" && path != "/" {
			exporterOpts = append(exporterOpts, otlptracehttp.WithURLPath(path))
		}

		switch strings.ToLower(strings.TrimSpace(parsed.Scheme)) {
		case "http":
			exporterOpts = append(exporterOpts, otlptracehttp.WithInsecure())
		case "https", "":
			// secure by default
		default:
			return nil, fmt.Errorf("invalid otlp endpoint %q: unsupported scheme %q", endpoint, parsed.Scheme)
		}
	} else {
		exporterOpts = append(exporterOpts, otlptracehttp.WithEndpoint(endpoint))
		if cfg.OTLP.Insecure {
			exporterOpts = append(exporterOpts, otlptracehttp.WithInsecure())
		}
	}

	exp, err := otlptracehttp.New(context.Background(), exporterOpts...)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(
		context.Background(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceName("llm-gateway"),
			semconv.ServiceVersion("dev"),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp, sdktrace.WithBatchTimeout(2*time.Second)),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
