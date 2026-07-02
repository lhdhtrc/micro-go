package telemetry

import (
	"context"
	"errors"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// Providers 聚合了 OpenTelemetry 的三个主要 Provider：
// - TracerProvider: 用于链路追踪
// - MeterProvider: 用于指标监控
// - LoggerProvider: 用于日志记录
type Providers struct {
	TracerProvider *sdktrace.TracerProvider
	MeterProvider  *sdkmetric.MeterProvider
	LoggerProvider *sdklog.LoggerProvider

	// MetricsHandler 是 Prometheus 的 HTTP Handler，
	// 用于对外暴露 /metrics 接口供 Prometheus Server 拉取数据。
	MetricsHandler http.Handler
}

// DefaultInitTimeout 是初始化 Telemetry 的默认超时时间。
const DefaultInitTimeout = 3 * time.Second

// NewProviders 创建并初始化 Telemetry Providers。
// 初始化过程使用 DefaultInitTimeout 作为默认超时控制。
func NewProviders(config *Config, source *Resource) (*Providers, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeout)
	defer cancel()

	p := &Providers{}

	// 1. 创建 Resource
	res, err := newResource(source)
	if err != nil {
		return nil, err
	}

	// 2. 设置全局 Propagator
	// 使用 W3C Trace Context 和 Baggage 标准
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	otlpEndpoint := config.OTLPEndpoint
	insecure := config.Insecure

	// 3. 初始化 Traces
	if config.Traces {
		tp, err := NewTracerProvider(ctx, res, otlpEndpoint, insecure)
		if err != nil {
			return nil, err
		}
		p.TracerProvider = tp
		// 设置全局 TracerProvider
		otel.SetTracerProvider(tp)
	}

	// 4. 初始化 Metrics
	if config.Metrics {
		mp, mh, err := NewMeterProvider(res)
		if err != nil {
			return nil, err
		}
		p.MeterProvider = mp
		p.MetricsHandler = mh
		// 设置全局 MeterProvider
		otel.SetMeterProvider(mp)
	}

	// 5. 初始化 Logs
	if config.Logs {
		lp, err := NewLoggerProvider(ctx, res, otlpEndpoint, insecure)
		if err != nil {
			return nil, err
		}
		p.LoggerProvider = lp
		// 设置全局 LoggerProvider
		global.SetLoggerProvider(lp)
	}

	return p, nil
}

// newResource 构造所有 OTel 信号共用的服务身份资源。
func newResource(source *Resource) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(source.ServiceName),
		semconv.ServiceVersion(source.ServiceVersion),
		semconv.ServiceNamespace(source.ServiceNamespace),
		semconv.ServiceInstanceID(source.ServiceInstanceId),
		attribute.String("service.id", source.ServiceId),
	}
	if source.Environment != "" {
		attrs = append(attrs, attribute.String("deployment.environment", source.Environment))
	}
	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, attrs...),
	)
}

func (p *Providers) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeout)
	defer cancel()

	var out error

	if p.LoggerProvider != nil {
		if err := p.LoggerProvider.Shutdown(ctx); err != nil {
			out = errors.Join(out, err)
		}
	}
	if p.MeterProvider != nil {
		if err := p.MeterProvider.Shutdown(ctx); err != nil {
			out = errors.Join(out, err)
		}
	}
	if p.TracerProvider != nil {
		if err := p.TracerProvider.Shutdown(ctx); err != nil {
			out = errors.Join(out, err)
		}
	}

	return out
}
