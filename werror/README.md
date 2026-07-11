# Wrap Error (`werror`)

`werror` 提供与传输协议无关的错误包装模型。业务代码用它表达“这是客户端输入错误、资源不存在、权限不足、服务不可用”等语义，gRPC/HTTP 出口再统一转换为协议状态。

## 服务错误目录

每个业务服务应在自己的 `internal/error` 包内集中定义错误，包名统一为 `ierror`，不要在 usecase/data/service 中重复写 code、reason 和默认消息：

```go
package ierror

import (
    "github.com/fireflycore/go-micro/werror"
    "google.golang.org/grpc/codes"
)

const Domain = "lhdht.auth"

var (
    TokenInvalid = werror.Definition{
        Code:    codes.Unauthenticated,
        Domain:  Domain,
        Reason:  "TOKEN_INVALID",
        Message: "令牌已失效，请重新登录",
    }

    Catalog = werror.MustCatalog(TokenInvalid)
)
```

业务代码通过定义创建错误：

```go
return ierror.TokenInvalid.New(werror.WithCause(err))
```

动态消息参数使用 metadata，不拼进 reason：

```go
return errors.RoleNotEmpty.New(
    werror.WithDetail("descendantCount", strconv.FormatInt(total, 10)),
)
```

`Catalog.Definitions()` 可用于导出服务错误目录、错误码查询页面和翻译完整性检查；目录初始化会拒绝非法 reason 和重复的 `(domain, reason)`。

## 直接构造

```go
import "github.com/fireflycore/go-micro/werror"

func validateCode() error {
    return werror.InvalidArgument(
        "验证码已过期/不存在",
        werror.WithDomain("lhdht.secure"),
        werror.WithReason("VERIFY_CODE_EXPIRED"),
    )
}
```

如果需要保留底层原因：

```go
import "google.golang.org/grpc/codes"

// ...

return werror.Wrap(
    codes.InvalidArgument,
    err,
    "验证码已过期/不存在",
    werror.WithDomain("lhdht.secure"),
    werror.WithReason("VERIFY_CODE_EXPIRED"),
)
```

## 跨协议错误详情

存在稳定 reason 时，`GRPCStatus()` 会附加标准 `google.rpc.ErrorInfo`；domain 和 metadata 随同 reason 输出。直接 gRPC 客户端和开启 `convert_grpc_status` 的 Envoy HTTP/JSON 转码都能读取：

```json
{
  "code": 16,
  "message": "令牌已失效，请重新登录",
  "details": [{
    "@type": "type.googleapis.com/google.rpc.ErrorInfo",
    "domain": "lhdht.auth",
    "reason": "TOKEN_INVALID",
    "metadata": {}
  }]
}
```

客户端使用 `(domain, reason)` 作为翻译缓存键，`message` 是立即展示的默认降级文案，metadata 用于命名参数替换。服务端错误路径不调用 multilingual；客户端按缓存命中、异步缺失队列和翻译包版本同步完成本地化。

## 语义边界

- 业务层负责判断错误语义，例如 `InvalidArgument`、`NotFound`、`PermissionDenied`。
- gRPC code 表达协议类别，`(domain, reason)` 表达稳定业务错误身份，两者不能混用。
- gRPC 出口统一使用 `middleware/grpc.ErrorToStatus` 转成 `status.Error`。
- 未分类普通错误默认应视为服务端错误，出口映射为 `codes.Internal`。
- 不建议靠字符串匹配决定错误状态；历史 sentinel 可临时通过 `gm.WithErrorMapping(...)` 迁移。

## 常用 code

| 场景 | 构造函数 |
| --- | --- |
| 参数、验证码、登录表单不合法 | `InvalidArgument` |
| 登录态缺失、token 无效 | `Unauthenticated` |
| 已认证但无权限 | `PermissionDenied` |
| 资源不存在 | `NotFound` |
| 资源重复 | `AlreadyExists` |
| 当前状态不允许操作 | `FailedPrecondition` |
| 下游服务不可用 | `Unavailable` |
| 未分类服务端错误 | `Internal` |
