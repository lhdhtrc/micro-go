# Middleware

`middleware` 目录包含了 firefly 微服务框架的各类中间件组件。

## 目录结构

- **[gRPC Middleware (gm)](./grpc/README.md)**: `middleware/grpc`
  - 提供 gRPC 服务端的拦截器与 OTel StatsHandler 适配，包括访问日志、错误映射、OTel 埋点入口等。

- **[HTTP Middleware (hm)](./http/log.go)**: `middleware/http`
  - 提供 HTTP 访问日志中间件（`NewAccessLogger`）。

## 快速导航

### gRPC Middleware (`gm`)

位于 `middleware/grpc` 包下。

主要功能：
- `NewAccessLogger`: 访问日志（结构化字段 + zap/otelzap 适配）。
- `ErrorToStatus`: 服务出口错误归一化，把 `werror`、`status.Error`、`protovalidate`、context error 和历史 sentinel 映射为 gRPC status。
- `NewOtelServerStatsHandler`: OTel gRPC Server StatsHandler（用于 trace/metrics 自动埋点）。

详细文档请参考：[grpc/README.md](./grpc/README.md)
