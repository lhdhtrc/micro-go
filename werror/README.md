# Wrap Error (`werror`)

`werror` 提供与传输协议无关的错误包装模型。业务代码用它表达“这是客户端输入错误、资源不存在、权限不足、服务不可用”等语义，gRPC/HTTP 出口再统一转换为协议状态。

## 推荐用法

```go
import "github.com/fireflycore/go-micro/werror"

func validateCode() error {
    return werror.InvalidArgument(
        "验证码已过期/不存在",
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
    werror.WithReason("VERIFY_CODE_EXPIRED"),
)
```

## 语义边界

- 业务层负责判断错误语义，例如 `InvalidArgument`、`NotFound`、`PermissionDenied`。
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
