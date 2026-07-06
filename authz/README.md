# Authz

`authz` 包提供业务服务接入 Firefly 数据面授权结果的通用配置与工具。

先区分四层对象：

- 传输层：跨进程只传 HTTP header / gRPC metadata。
- authority：`X-Firefly-User-Authority` / `X-Firefly-Service-Authority`，只给 authz 校验和解析。
- 签名层：`x-firefly-authz-sign` 是 authz allow 后写入的 compact JWS。
- 进程内：`service.Context` 和 `service.AuthzSign` 是业务服务本地结构体，不跨进程传输。

它负责：

- 定义服务侧验签配置结构体 `VerificationConfig`
- 加载 Ed25519 公钥 PEM
- 构造 `service.AuthzSignVerificationOptions`
- 返回 gRPC middleware 需要的 `AuthzVerification` 与 `AuthzSkipMethods`
- 定义 `ServiceAuthorityProvider` / ServiceToken 管理器，供出站调用每一跳覆盖 `X-Firefly-Service-Authority`
- 提供 gRPC client interceptor 和 metadata helper，统一保留用户 authority 与短 TTL authz sign，清理普通身份 metadata 和未知业务 metadata

`x-firefly-authz-sign` 是服务侧验签输入。业务服务开启 `AuthzVerification` 后，`service.Context.VerifiedAuthzSign` 保存验签后的 JWS payload，`service.Context.ApiMethod` / `service.Context.ApiPath` 以该 payload 为可信来源。

`service.Context.AppId` 只表示用户身份中的 app_id；当前这一跳的调用方应用 ID 使用 `service.Context.InvokeAppId`。`service.Context.ServiceAppId / ServiceInstanceId` 只表示当前业务服务自身身份，用于本地日志、OTel 和数据库链路排障，不参与 authz 权限元组。新代码优先使用 `service.Context.UserContext`、`InvokeServiceAppId`、`TargetServiceAppId` 和 `DecisionContext`。

业务服务只需要在启动配置中声明：

```json
{
  "authz_verification": {
    "kid": "default",
    "public_key_path": "/etc/firefly/authz/keys/default-pub.pem",
    "issuer": "firefly-authz",
    "clock_skew": "5s",
    "skip_methods": [
      "/grpc.health.v1.Health/Check",
      "/grpc.health.v1.Health/Watch"
    ]
  }
}
```

`authz_verification` 配置存在时必须提供 `public_key_path`。是否对某个服务入口启用验签由服务启动装配决定，不再通过 `enabled` 字段做运行时开关。

启用验签时，`ServiceContextUnaryInterceptor` 必须传入当前服务 `ServiceAppId`。服务侧本地验签会用该值校验 `AuthzSign.target_app_id`，防止其他服务的授权结果被跨服务复用。

然后在 gRPC server 初始化时接入：

```go
verification, err := authz.NewVerificationOptions(bootstrapConfig.AuthzVerification)
if err != nil {
    panic(err)
}

gm.NewServiceContextUnaryInterceptor(gm.ServiceContextInterceptorOptions{
    ServiceAppId:      bootstrapConfig.App.Id,
    ServiceInstanceId: bootstrapConfig.App.InstanceId,
    AuthzVerification: verification.AuthzVerification,
    AuthzSkipMethods:  verification.AuthzSkipMethods,
})
```

当前只支持 EdDSA/Ed25519 JWS。`kid` 和 `issuer` 的默认值属于 `authz` 包内部配置语义，不放入 `constant` 公共常量。

## Service Authority

服务间调用需要同时遵守两条规则：

- 入站存在 `X-Firefly-User-Authority` 时继续透传，保持用户身份上下文。
- 每一跳由当前服务覆盖 `X-Firefly-Service-Authority`，表达当前调用方服务身份。

推荐在启动期把 auth 服务的 `GenerateServiceToken` 封装为 fetch 函数，并启动后台刷新：

```go
provider, err := authz.NewServiceAuthorityProvider(
    &authz.ServiceAuthorityConfig{
        RefreshBefore: "1m",
    },
    func(ctx context.Context) (*authz.ServiceAuthorityToken, error) {
        resp, err := authTokenClient.GenerateServiceToken(ctx, req)
        if err != nil {
            return nil, err
        }
        return authz.NewServiceAuthorityToken(resp.Data.Token, resp.Data.Expired)
    },
)
if err != nil {
    panic(err)
}
if err := provider.Start(ctx); err != nil {
    panic(err)
}
defer provider.Stop()

invoker := invocation.NewUnaryInvoker(manager, timeout).
    WithServiceAuthorityProvider(provider)
```

`ServiceAuthorityProvider` 会在进程内缓存 service token，并在后台按 `RefreshBefore` 主动刷新。首次 fetch 会在 `Start(ctx)` 后立即异步执行；失败后按 `min(1 minute * retry_count * 10, 60 minutes)` 退避并无限次重试，成功后清零。没有有效 service token 时，出站 Firefly 服务调用返回 `ErrServiceTokenUnavailable`，不会只携带用户 token 穿透下游。

管理端或排障接口可通过 `ServiceAuthorityStatus()` 读取安全状态快照。该快照只包含 `started`、`loaded`、`expires_at`、`expires_in_seconds`、`refresh_before` 和 `last_error`，不包含 service token 原文，适合挂到业务服务 `/info` 这类自管理接口。

出站 metadata 采用白名单策略，保留用户 authority、短 TTL `x-firefly-authz-sign`、OTel trace/baggage 和访问日志需要的客户端事实；普通身份 metadata、当前服务自身 metadata、上一跳 service authority 以及未知业务 metadata 会被清理。下一跳 authz 可以验签复用身份解析结果，但仍必须基于当前 route 重新做权限判定并重新签发新的 `x-firefly-authz-sign`。
