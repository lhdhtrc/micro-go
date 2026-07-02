# Telemetry（OpenTelemetry）接入说明

本目录提供了 OpenTelemetry (OTel) 的一站式初始化封装，用于在服务启动时一次性完成 **Traces / Metrics / Logs** 的配置，并设置全局 Provider 供其他库复用。

## 目录结构

- [core.go](core.go): 初始化入口 (`NewProviders`) 与统一关闭 (`Providers.Shutdown`)
- [trace.go](trace.go): 链路追踪 (`TracerProvider`) 配置
- [metric.go](metric.go): 指标监控 (`MeterProvider`) 配置
- [log.go](log.go): 日志 (`LoggerProvider`) 配置
- [config.go](config.go): `Config` 与 `Resource` 结构定义

## 1. OTel 在做什么

OTel 的通用工作模型是：

1. 业务代码或三方库产生信号（Trace/Metric/Log）。
2. 信号交给对应 Provider（TracerProvider/MeterProvider/LoggerProvider）。
3. Provider 通过 Processor/Reader/Exporter 把数据导出到后端（Collector、Prometheus、Tempo、Loki 等）。

其中：

- Trace：`TracerProvider` + `SpanProcessor` + `SpanExporter`
- Metric：`MeterProvider` + `Reader`（Prometheus 是 pull reader）
- Log：`LoggerProvider` + `LogProcessor` + `LogExporter`

## 2. telemetry.NewProviders 做了哪些事

[NewProviders](core.go) 分成 5 个关键步骤：

### 2.1 Resource：统一标识“这是谁发出来的数据”

`telemetry.NewProviders` 现在不再直接依赖业务侧 `BootstrapConfig`、`app.Config` 或 `service.Config`，而是只接收本库所需的最小输入：

```go
type Resource struct {
    ServiceId         string
    Environment       string
    ServiceName       string
    ServiceVersion    string
    ServiceNamespace  string
    ServiceInstanceId string
}
```

内部会基于这组字段构造 OTel Resource：

```go
resource.Merge(
  resource.Default(),
  resource.NewWithAttributes(
    semconv.SchemaURL,
    semconv.ServiceName(source.ServiceName),
    semconv.ServiceVersion(source.ServiceVersion),
    semconv.ServiceNamespace(source.ServiceNamespace),
    semconv.ServiceInstanceID(source.ServiceInstanceId),
    attribute.String("service.id", source.ServiceId),
    attribute.String("deployment.environment", source.Environment),
  ),
)
```

这部分是所有信号的共同“身份标签”。`resource.Default()` 会保留 OTel 默认资源字段（如 `service.name` 默认值与 `telemetry.sdk.*`），并叠加服务标准语义字段（`service.name/service.version/service.namespace/service.instance.id`）和项目自定义字段（`service.id`）。
字段语义建议为：`service.id` 表示服务父级标识（对应 `app_id`），`service.instance.id` 表示该服务下的具体实例，`deployment.environment` 表示服务运行环境；平台检索与聚合优先使用标准字段 `service.name/service.namespace/service.instance.id`，`service.id` 作为业务维度补充字段。

### 2.2 Propagator：跨进程传播 Trace 上下文

```go
otel.SetTextMapPropagator(
  propagation.NewCompositeTextMapPropagator(
    propagation.TraceContext{},
    propagation.Baggage{},
  ),
)
```

这意味着链路上下文使用 W3C TraceContext（`traceparent`）标准格式传播。

### 2.3 Traces：OTLP/gRPC 导出到 Collector

当 `config.Traces=true`：

1. 创建 OTLP Trace exporter（gRPC）
2. 创建 `sdktrace.TracerProvider`，使用 `WithBatcher` 批量异步导出
3. 设置全局 tracer provider：`otel.SetTracerProvider(...)`

### 2.4 Metrics：Prometheus pull 模式暴露 /metrics

当 `config.Metrics=true`：

1. 创建 Prometheus registry（隔离 default registry）
2. 创建 OTel Prometheus exporter，并注册到 registry
3. 创建 `sdkmetric.MeterProvider`，`WithReader(metricExp)`
4. 设置全局 meter provider：`otel.SetMeterProvider(...)`
5. 暴露 `providers.MetricsHandler`（`promhttp.HandlerFor(reg, ...)`）

Prometheus 会通过 HTTP 拉取 `/metrics`，所以这里 exporter 不是“push”，而是**提供一个可被 scrape 的视图**。

### 2.5 Logs：OTLP/gRPC 导出到 Collector

当 `config.Logs=true`：

1. 创建 OTLP Log exporter（gRPC）
2. 创建 `sdklog.LoggerProvider`，使用 batch processor 异步导出
3. 设置 Logs 全局 provider：`global.SetLoggerProvider(...)`

## 3. telemetry 如何与 zap 协作（otelzap）

`otelzap` 是 zap 的 bridge：它实现了 `zapcore.Core`，zap 写日志时会调用 core 的 `Write`，`otelzap` 会把 zap 的 `Entry/Fields` 转成 OTel `log.Record` 并发给 OTel 的 `LoggerProvider`。

在 go-micro 中，zap 构造在 [logger.NewZapLogger](../logger/zap.go)：

- `config.Console=true`：追加 console core，输出到 stdout
- `config.Remote=true`：追加 `otelzap.NewCore(...)`，输出到 OTel

### 3.1 Trace/Span 关联是怎么做到的

`otelzap` 有一个关键约定：如果 zap fields 里包含 `context.Context`，会用它作为 log record 的上下文，从中读取当前 span，从而自动把日志关联到 trace/span。

go-micro 的 `logger.AccessLogger` 与 `logger.ServerLogger` 提供了 `WithContextInfo/WithContextWarn/WithContextError`：

```go
func (l *AccessLogger) WithContextInfo(ctx context.Context, msg string, fields ...zap.Field) {
  l.Info(msg, append(fields, zap.Any("ctx", ctx))...)
}
```

因此在请求链路内打日志时，只要传入当前 ctx：

```go
log.WithContextInfo(ctx, "something happened", zap.String("k", "v"))
```

`otelzap` 就能把这条日志自动挂到当前 trace 上。

## 4. telemetry 如何与 gRPC 协作（otelgrpc）：指标与追踪

### 4.1 gRPC 指标：StatsHandler

go-micro 提供了 [gm.NewOtelServerStatsHandler]：

```go
grpc.NewServer(
  grpc.StatsHandler(gm.NewOtelServerStatsHandler()),
)
```

原理是：

- gRPC 内部会把每次 RPC 的开始/结束/字节数等事件回调给 `stats.Handler`
- `otelgrpc.NewServerHandler` 在这些回调中使用全局 `MeterProvider` 记录指标
- 这些指标进入 `telemetry.NewProviders` 创建的 `MeterProvider`，通过 `providers.MetricsHandler` 暴露给 Prometheus scrape

因此，“gRPC 指标是 otelgrpc 采集的”，而 telemetry 做的是“提供 meter provider + 暴露 /metrics 出口”。

### 4.2 gRPC Trace：StatsHandler

在当前版本的 `otelgrpc` 中，推荐通过 `stats.Handler` 完成 trace/metrics 采集，不再使用已弃用的拦截器方式。你可以在服务中使用：

```go
grpc.NewServer(
  grpc.StatsHandler(gm.NewOtelServerStatsHandler()),
  grpc.ChainUnaryInterceptor(
    gm.NewAccessLogger(log),
    gm.ValidationErrorToInvalidArgument(),
  ),
)
```

建议把 `grpc.StatsHandler(gm.NewOtelServerStatsHandler())` 作为服务级配置，保证中间件中的日志可以从 `ctx` 中拿到 span 做关联。

## 5. 推荐接入模板（最小可运行骨架）

### 5.1 启动时初始化 telemetry

```go
providers, err := telemetry.NewProviders(&conf.Telemetry, &telemetry.Resource{
  ServiceId:         conf.App.Id,
  Environment:       conf.App.Env,
  ServiceName:       conf.Service.Service,
  ServiceVersion:    conf.App.Version,
  ServiceNamespace:  conf.Service.Namespace,
  ServiceInstanceId: conf.App.InstanceId,
})
if err != nil { panic(err) }
defer func() { _ = providers.Shutdown() }()
```

### 5.2 暴露 /metrics

#### 方案：独立 HTTP 端口 (推荐)

**不推荐**与 gRPC 业务端口复用（如使用 `cmux`），以避免潜在的性能损耗和排查复杂度。

建议开启一个独立的 HTTP 端口（例如 `9091`），用于暴露 Metrics 和 Health Check。这也是 Consul 等注册中心进行健康检查的标准做法。

```go
// 1. 创建 HTTP ServeMux
mux := http.NewServeMux()

// 2. 注册 Metrics 路由
mux.Handle("/metrics", providers.MetricsHandler)

// 3. 注册 Health Check 路由 (供 Consul 调用)
// Consul 默认会 ping 这个接口，返回 200 OK 即为健康
mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
})

// 4. 启动 HTTP Server
go func() {
    // 建议端口号：gRPC端口 + 1
    if err := http.ListenAndServe(":9091", mux); err != nil {
        panic(err)
    }
}()
```

### 5.3 构造 logger（Console + OTel Remote）

```go
zl := logger.NewZapLogger(conf.App.Name, &conf.Logger)
log := logger.NewAccessLogger(zl)
```

### 5.4 gRPC Server：StatsHandler + Interceptor

```go
s := grpc.NewServer(
  grpc.StatsHandler(gm.NewOtelServerStatsHandler()),
  grpc.ChainUnaryInterceptor(
    gm.NewAccessLogger(log),
    gm.ValidationErrorToInvalidArgument(),
  ),
)
_ = s
```

两者的职责区别：

- `grpc.StatsHandler(gm.NewOtelServerStatsHandler())`
  - 属于 gRPC 的 `stats.Handler` 回调机制
  - 主要负责 Metrics：采集 RPC 过程中的统计事件（开始/结束/字节数等）并记录为指标，最终通过 `/metrics` 暴露给 Prometheus scrape
- `grpc.ChainUnaryInterceptor(...)`
  - 属于 gRPC 的 Unary 拦截器链
  - 主要负责业务能力增强：参数校验错误映射、访问日志落库/输出等

## 6. 常见排查点

- 在 Tempo 看不到 trace：
  - 是否启用了 `config.Traces=true`
  - 是否挂了 `grpc.StatsHandler(gm.NewOtelServerStatsHandler())` 或者其它 instrumentation
  - OTLP endpoint 是否可达
- /metrics 没有 gRPC 指标：
  - 是否启用了 `config.Metrics=true`
  - 是否挂了 `grpc.StatsHandler(gm.NewOtelServerStatsHandler())`
  - Prometheus 是否 scrape 到正确端口/路径
- 日志无法和 trace 关联：
  - 打日志时是否把 `ctx` 传进 `WithContextInfo/WithContextError`
  - 是否确实存在当前 span（是否装了 `grpc.StatsHandler(gm.NewOtelServerStatsHandler())`）

## 7. 推荐装配方式

推荐把配置聚合和字段映射放在业务服务启动层完成，而不是让 `telemetry` 直接依赖业务侧配置模型：

```go
type BootstrapConfig struct {
  App struct {
    Id         string `json:"id"`
    Name       string `json:"name"`
    Version    string `json:"version"`
    InstanceId string `json:"instance_id"`
  } `json:"app"`

  Service struct {
    Service   string `json:"service"`
    Namespace string `json:"namespace"`
  } `json:"service"`

  Telemetry telemetry.Config `json:"telemetry"`
}
```

在组合根中做一次映射：

```go
source := &telemetry.Resource{
  ServiceId:         conf.App.Id,
  Environment:       conf.App.Env,
  ServiceName:       conf.Service.Service,
  ServiceVersion:    conf.App.Version,
  ServiceNamespace:  conf.Service.Namespace,
  ServiceInstanceId: conf.App.InstanceId,
}

providers, err := telemetry.NewProviders(&conf.Telemetry, source)
```

这样做的目的不是减少字段，而是稳定边界：

- 业务服务继续拥有自己的启动配置模型
- `telemetry` 只消费自己所需的运行时资源信息
- `app`、`service` 等包的结构调整不会直接扩散到 `telemetry` 初始化签名
