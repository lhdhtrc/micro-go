# gRPC Middleware (gm)

`gm` 包提供了构建 gRPC 微服务所需的通用中间件（Interceptor）与 OpenTelemetry StatsHandler 适配。

## 功能列表

### 1. Service Context (`NewServiceContextUnaryInterceptor`)

在请求入口统一完成：

- 注入当前业务服务自身的 `ServiceAppId / ServiceInstanceId`
- 解析入站 metadata 中的普通身份字段和 authz compact JWS
- 构造服务内唯一主上下文 `service.Context`
- 从当前 OTel span 提取 trace 标识快照
- 按需本地验签 `x-firefly-authz-sign`

**推荐用途**：
- 在 gRPC 服务端入口统一注入 `service.Context`
- access log 与后续审计扩展统一读取字段
- 在请求入口统一完成服务内主上下文注入

`gm` 当前只负责服务端入站中间件语义，不再定义服务内主上下文模型；业务代码应从 `go-micro/service` 读取 `service.Context`，出站调用统一由 `go-micro/invocation` 直接基于当前 gRPC context 与 OTel trace 处理。

`service.Context.AppId` 只表示用户身份中的 app_id；当前这一跳调用方应用 ID 使用 `InvokeAppId`，被访问服务应用 ID 使用 `TargetAppId`。

`ServiceAppId / ServiceInstanceId` 是当前业务服务自身身份，通常来自 `bootstrapConfig.App.Id` 和 `bootstrapConfig.App.InstanceId`。它们只服务本地日志、OTel 和 gormx 等组件，不参与 authz 授权元组，也不会随出站调用透传。

启用 `AuthzVerification` 时必须提供 `ServiceAppId`。服务侧验签会用它绑定 `AuthzSign.target_app_id`，避免把其他服务的 allow 结果复用到当前服务。

### 2. Access Logger (`NewAccessLogger`)

提供 gRPC 访问日志记录功能，输出结构化字段（zap fields）。

**特性**：
- **链路关联**：通过 `otelzap` 从 `ctx` 自动关联 trace（要求服务端启用 OTel stats handler，日志使用 `zap.Any("ctx", ctx)`）。
- **身份识别**：优先读取进程内 `service.Context`，必要时只回退读取普通身份 metadata，不从未签名资源字段推导授权动作和路径。
- **性能字段**：`duration`（微秒）、`status`（gRPC code）、`path` 等。
- **错误聚合**：结构化错误额外记录 `error_domain / error_reason`；ErrorInfo metadata 默认不写日志。

**用法**：

```go
import (
	"github.com/fireflycore/go-micro/logger"
	"github.com/fireflycore/go-micro/middleware/grpc" // 别名通常为 gm
	"google.golang.org/grpc"
)

// 创建 gRPC Server 时注入
accessLog := logger.NewAccessLogger(zl)
s := grpc.NewServer(
	grpc.UnaryInterceptor(gm.NewAccessLogger(accessLog)),
)
```

### 3. 出口错误映射 (`ErrorToStatus`)

`ErrorToStatus` 是推荐的 gRPC 服务出口错误归一化中间件，用于把业务错误和框架错误统一转成 gRPC status：

- 已经是 `status.Error` 或实现 `GRPCStatus()` 的错误会保持原语义。
- `werror` 业务错误会按自身携带的 gRPC code 返回，并保留标准 `google.rpc.ErrorInfo` 中的 domain、reason 和 metadata。
- `protovalidate.ValidationError` 默认映射为 `codes.InvalidArgument`。
- `context.Canceled` / `context.DeadlineExceeded` 分别映射为 `codes.Canceled` / `codes.DeadlineExceeded`。
- 历史 sentinel error 可通过 `WithErrorMapping(...)` 显式映射。
- 未分类普通 error 默认映射为 `codes.Internal`，避免泄漏为 `codes.Unknown`。

业务代码推荐使用 `github.com/fireflycore/go-micro/werror` 表达错误语义：

```go
return werror.InvalidArgument(
    "验证码已过期/不存在",
    werror.WithDomain("lhdht.secure"),
    werror.WithReason("VERIFY_CODE_EXPIRED"),
)
```

临时兼容历史 sentinel：

```go
gm.ErrorToStatus(
    gm.WithErrorMapping(ErrVerifyCodeExpired, codes.InvalidArgument, "验证码已过期/不存在"),
)
```

如果生产环境不希望把未分类错误原文返回给客户端，可配置：

```go
gm.ErrorToStatus(
    gm.WithExposeDefaultErrorMessage(false),
    gm.WithDefaultErrorMessage("internal server error"),
)
```

### 4. OpenTelemetry gRPC 埋点（StatsHandler）

`NewOtelServerStatsHandler` 返回 `stats.Handler`，用于 `grpc.StatsHandler(...)` 挂载到服务端，自动完成 trace/metrics 采集与 W3C `traceparent` 传播。

## 组合使用

通常建议使用 `grpc.ChainUnaryInterceptor` 组合多个中间件：

```go
s := grpc.NewServer(
    grpc.StatsHandler(gm.NewOtelServerStatsHandler()),
    grpc.ChainUnaryInterceptor(
        gm.NewServiceContextUnaryInterceptor(gm.ServiceContextInterceptorOptions{
            ServiceAppId:      bootstrapConfig.App.Id,
            ServiceInstanceId: bootstrapConfig.App.InstanceId,
            // 生产环境建议配置 AuthzVerification，让服务侧信任验签后的 JWS payload。
            // AuthzVerification: &service.AuthzSignVerificationOptions{...},
        }),
        gm.NewAccessLogger(accessLog),
        gm.ErrorToStatus(),
    ),
)
```

`ErrorToStatus` 建议放在 `NewAccessLogger` 之后作为更内层的出口中间件。这样业务 handler 返回的错误会先归一化为最终 gRPC code，再被访问日志记录，避免日志中的 `status` 仍显示未归一化的 `Unknown`。
