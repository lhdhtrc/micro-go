# Service

`service` 包定义服务内统一主上下文模型。

`service.Context` 是业务服务进程内结构体，不是跨进程传输对象。跨进程只传 HTTP header / gRPC metadata；入口中间件把这些 metadata 结构化成本地 `service.Context`。

它只负责：

- 定义 `service.Context`
- 提供 `WithContext(...)` / `FromContext(...)` / `MustFromContext(...)`
- 提供 `BuildContext(...)` 把入站 metadata 与当前 OTel span 结构化为服务内主上下文
- 提供 `VerifyAuthzSign(...)` / `BuildVerifiedContext(...)` 对 `x-firefly-authz-sign` JWS 做本地验签
- 定义 `RequestScope`、`AuthorizedScope` 和按 gRPC FullMethod 注册的 `RequestScopePolicyRegistry`
- 提供 `ExtractRequestScope(...)` 与 `RequestScopeFromContext(...)`，承载进程内请求数据作用域

它不负责：

- 远程业务服务 DNS 建模
- gRPC interceptor 装配
- 出站 metadata 拼装
- 下游服务调用
- token 解析或在线权限判断
- LHDHT 等产品域的数据权限事实和授权策略

当前推荐分工：

- `middleware/grpc`：在 gRPC 服务端入口创建并注入 `service.Context`
- `service`：供 `service / biz / data / log` 统一读取服务内主上下文，并提供 authz JWS 本地验签能力
- `invocation`：负责远程业务服务 DNS 与纯出站调用语义

`service.Context` 中的 `AppId` 只表示用户身份中的应用 ID；没有用户身份时可以为空。本跳权限判定中的调用方应用 ID 使用 `InvokeAppId`，route 所属服务 app_id 在 authz 判定中表现为 `TargetAppId`。

`service.Context.ServiceAppId` / `ServiceInstanceId` 表示当前业务服务自身身份，由入口中间件从本地 bootstrap 配置注入。它们用于本地日志、OTel 和数据库链路排障，不是 `InvokeServiceAppId`，不参与 authz 权限元组，也不会被出站调用透传给下一跳。

新代码建议优先读取分组字段：

- `UserContext`：用户身份，字段保持 `user_id / app_id / tenant_id / session / org_ids / post_ids / role_ids`
- `InvokeServiceAppId`：当前这一跳调用方服务 app_id，只在服务主体场景存在，来源于 authz 对服务 authority 的解析
- `TargetServiceAppId`：当前这一跳被访问服务 app_id，来源于 authz 对 route.app_id 的映射
- `DecisionContext`：authz allow 后的判定事实

扁平字段仍然保留，方便日志和业务读取，但不要把 `AppId`、`InvokeAppId` 和 `InvokeServiceAppId` 混用。`InvokeAppId` 是授权元组里的调用方应用 ID；用户首跳时它可以来自 `UserContext.app_id`，此时不代表存在调用服务。

服务权限粒度当前固定到 app_id 级别，`service.Context` 不再保留 `InvokeInstanceId` / `TargetInstanceId`。当前服务自身实例标识仍可通过 `ServiceInstanceId` 进入本地上下文，也可用于服务注册、健康检查和 OTel resource，但不进入 authz 权限元组。

`service.Context.AuthzSignJWS` 保存原始 `x-firefly-authz-sign` compact JWS；`service.Context.VerifiedAuthzSign` 保存验签后的 payload。

`service.Context.RouteMethod` / `RoutePath` 只在 `BuildVerifiedContext(...)` 本地验签成功后可信；普通 metadata 只作为读取便利，不是信任根。验签后的 `AuthzSign` 必须携带结构化 `user_context`，以及字符串字段 `invoke_service_app_id` / `target_service_app_id`，不再接受旧平铺身份 payload。

## 请求数据作用域

请求数据作用域与登录上下文、接口授权上下文相互独立：

- `service.Context.AppId/TenantId` 表示用户登录身份，不能被业务请求参数覆盖。
- `RequestScope` 表示 RPC 显式携带的可选数据范围。
- `AuthorizedScope` 表示产品域授权器批准后的应用、租户集合或全量维度。

只有注册到 `RequestScopePolicyRegistry` 的 RPC 才会把顶层 `app_id/tenant_id` 解释为请求作用域。未登记方法中的同名字段仍是普通业务字段。`RequestScopeContext` 只在当前进程内使用，不转换为 metadata，也不自动传播到下游服务。
