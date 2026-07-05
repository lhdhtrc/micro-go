package telemetry

import (
	"net/http"

	promreg "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// NewMeterProvider 初始化 MeterProvider 并配置 Prometheus 导出器。
//
// 参数:
//   - res: 资源属性（包含服务名、版本等）
//
// 返回:
//   - *sdkmetric.MeterProvider: 配置好的 MeterProvider
//   - http.Handler: 用于暴露 /metrics 的 HTTP Handler
//   - error: 初始化过程中的错误
func NewMeterProvider(res *resource.Resource) (*sdkmetric.MeterProvider, http.Handler, error) {
	// 创建一个新的 Prometheus Registry，避免使用全局默认 Registry
	reg := promreg.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	// 创建 OTel Prometheus Exporter
	metricExp, err := otelprom.New(
		otelprom.WithRegisterer(reg),
	)
	if err != nil {
		return nil, nil, err
	}

	// 创建 MeterProvider，关联 Resource 和 Reader
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(metricExp),
	)

	// 返回 MeterProvider 和对应的 HTTP Handler
	return mp, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}), nil
}
