# Firefly Go Micro

`go-micro` 是 Firefly 微服务框架的 Go 版本核心库，提供了构建微服务所需的基础设施抽象与通用工具。

建议配合 **go-layout**（Firefly 微服务框架的 Go 版本标准项目模板）使用，以获得最佳开发体验。

当前版本已经收敛为两条明确路径：

- 裸机接入路径：`go-consul/agent`，负责对接本机 `sidecar-agent`
- 服务调用路径：`invocation`，面向 `service -> service` 的统一调用模型

旧 `registry` 体系文档已经迁入 design 仓库的 `design/registry` 目录。

同时，`go-micro` 已明确采用 OpenTelemetry 作为统一观测体系：

- gRPC 调用链默认接入 OTel
- 指标、链路、日志相关能力围绕 OTel 生态组织
- `invocation` 的后续实现也应默认对齐这一观测模型

在权限链路上，`go-micro` 只负责两件事：把入站 header/metadata 结构化为进程内 `service.Context`，以及在服务显式配置后本地验签 authz 写入的 compact JWS。

- token 解析和权限判定由 authz 数据面完成
- 跨进程只传 HTTP header / gRPC metadata，不传 `UserContext` 或 `AuthzSign` 结构体
- `X-Firefly-User-Authority` / `X-Firefly-Service-Authority` 是传给 authz 校验的 authority 原文
- `x-firefly-authz-sign` 是 authz 签名后的 compact JWS；业务服务验签后才得到 `VerifiedAuthzSign`
- `service.Context` 是业务服务进程内结构体，由 metadata 和可选 JWS 验签结果组装
- `service.Context.AppId` 只表示用户身份中的 app_id；本跳调用方应用 ID 使用 `InvokeAppId`
- `service.Context.ServiceAppId / ServiceInstanceId` 只表示当前服务自身身份，用于本地日志、OTel 和数据库链路排障，不参与 authz 授权元组
- 出站调用通过 `invocation` 透传用户 authority，并通过 `authz.ServiceAuthorityProvider` 由当前服务覆盖服务 authority
- ServiceToken 管理器在启动后后台异步获取和刷新 token；获取失败不阻塞服务启动，缺少有效 token 时下游 Firefly 服务调用返回不可用错误
- 出站调用会保留用户 authority 和短 TTL authz sign，清理普通身份 metadata，避免复用上一跳普通上下文或服务身份

## 安装

```bash
go get github.com/fireflycore/go-micro
```

## 快速开始

以 gRPC 服务为例，常见用法是把中间件注入到 `grpc.Server`，并挂载 OpenTelemetry 的 gRPC StatsHandler：

```go
import (
	"github.com/fireflycore/go-micro/service"
	"github.com/fireflycore/go-micro/middleware/grpc" // 别名通常为 gm
	"google.golang.org/grpc"
)

s := grpc.NewServer(
	grpc.StatsHandler(gm.NewOtelServerStatsHandler()),
	grpc.ChainUnaryInterceptor(
		gm.NewServiceContextUnaryInterceptor(gm.ServiceContextInterceptorOptions{
			ServiceAppId:      "auth",
			ServiceInstanceId: "auth-instance-1",
			AuthzVerification: &service.AuthzSignVerificationOptions{
				Issuer: "firefly-authz",
				// PublicKeys: map[string]ed25519.PublicKey{"default": authzPublicKey},
			},
		}),
		gm.NewAccessLogger(log),
		gm.ErrorToStatus(),
	),
)
_ = s
```

业务方法推荐用 `werror` 表达错误语义，再由 `gm.ErrorToStatus()` 在出口统一转成 gRPC status：

```go
return nil, werror.InvalidArgument(
    "验证码已过期/不存在",
    werror.WithReason("VERIFY_CODE_EXPIRED"),
)
```

`gm.ErrorToStatus()` 建议放在 `gm.NewAccessLogger(...)` 之后作为更内层的拦截器，这样访问日志记录的是归一化后的最终 gRPC code。

如果要面向新的服务调用模型，建议优先使用 `invocation` 包提供的能力：

- 用 `DNS` 表达“我要调用哪个业务服务 DNS”
- 用 `DNSManager` 统一组装标准 gRPC target
- 用 `ConnectionManager` 统一管理 `grpc.ClientConn`
- 用 `RemoteServiceManaged / RemoteServiceCaller / UnaryInvoker` 统一串起 metadata、连接复用与底层调用
- 用 `authz.ServiceAuthorityProvider` / ServiceToken 管理器统一接入 auth 服务签发的 service token
- 并默认把调用链路接入 OTel 观测体系

适用场景：

- `go-k8s + K8s + Istio` 标准主路径
- `go-consul + consul + envoy + sidecar-agent` 裸机主路径
- 需要统一调用入口而不是直接操作节点列表的场景

## 模块说明

详细文档请参考各子包目录下的 README：

- [invocation](./invocation/README.md)：新的服务调用模型（推荐）
- [werror](./werror/README.md)：业务错误语义与 gRPC status 映射基础
- [service](./service/README.md)：服务内统一主上下文模型
- [go-consul/agent](/Users/lhdht/product/synergy/firefly/golang/go-consul/agent/README.md)：业务服务与本机 sidecar-agent 的联动桥接
- [middleware](./middleware/README.md)：中间件（gRPC/HTTP）
- [logger](./logger/README.md)：zap/otelzap 日志封装
- [constant](./constant/README.md)：通用常量

## 当前建议

- 新项目优先围绕 `invocation` 设计服务间调用
- 裸机业务服务统一通过 `go-consul/agent` 接入 sidecar-agent
- 新增的客户端接入统一通过 `invocation` 提供的连接、metadata 与调用能力落地
- 新的调用能力默认应与 `telemetry`、`middleware/grpc` 中现有的 OTel 能力保持一致

## 许可证

[LICENSE](./LICENSE)
