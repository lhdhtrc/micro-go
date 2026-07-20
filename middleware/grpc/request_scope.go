package gm

import (
	"context"
	"time"

	"github.com/fireflycore/go-micro/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// RequestScopeInterceptorOptions 定义请求作用域一元拦截器的运行依赖。
type RequestScopeInterceptorOptions struct {
	// Registry 保存按 gRPC FullMethod 显式登记的请求作用域策略。
	Registry *service.RequestScopePolicyRegistry
	// Authorizer 负责调用产品域能力判断显式作用域是否被允许。
	Authorizer service.RequestScopeAuthorizer
	// Now 用于注入当前时间，生产环境默认使用 time.Now，测试可以固定时间。
	Now func() time.Time
}

// NewRequestScopeUnaryInterceptor 创建请求作用域提取和授权拦截器。
// 该拦截器只处理显式选择范围，接口默认范围和最终数据权限仍由业务服务负责。
func NewRequestScopeUnaryInterceptor(options RequestScopeInterceptorOptions) grpc.UnaryServerInterceptor {
	// 优先使用调用方注入的时间函数，便于稳定测试决策有效期。
	now := options.Now
	// 未注入时使用系统当前时间。
	if now == nil {
		now = time.Now
	}
	// 返回符合 gRPC UnaryServerInterceptor 签名的闭包。
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// 业务处理器缺失属于服务端装配错误，必须按失败关闭原则拒绝执行。
		if handler == nil {
			return nil, status.Error(codes.Internal, "request scope handler is missing")
		}
		// 没有登记表或方法信息时不解释请求中的同名业务字段。
		if options.Registry == nil || info == nil {
			return handler(ctx, req)
		}

		// 使用当前 RPC 的 gRPC FullMethod 查询显式登记策略。
		policy, ok := options.Registry.Lookup(info.FullMethod)
		// 未登记或明确为 NONE 的 RPC 直接进入业务处理。
		if !ok || policy.Mode == service.RequestScopeModeNone {
			return handler(ctx, req)
		}
		// 已登记 RPC 必须使用 protobuf 请求，才能按字段描述安全提取作用域。
		message, ok := req.(proto.Message)
		if !ok {
			return nil, status.Error(codes.Internal, "registered request scope method must use a protobuf request")
		}
		// 只按该 RPC 允许的维度提取顶层 app_id 和 tenant_id。
		requested, err := service.ExtractRequestScope(message, policy)
		// 请求字段契约错误统一映射为 InvalidArgument。
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		// 无论是否显式选择范围，都把本 RPC 的作用域语义写入进程内上下文。
		scopeContext := &service.RequestScopeContext{
			FullMethod: info.FullMethod,
			Policy:     policy,
			Requested:  requested,
		}
		// 只有调用方显式传入非空范围时才触发产品域授权判定。
		if requested.Explicit() {
			// 显式作用域没有授权器时禁止继续执行，避免绕过权限检查。
			if options.Authorizer == nil {
				return nil, status.Error(codes.FailedPrecondition, "request scope authorization is unavailable")
			}
			// 把当前方法、作用域语义和请求值完整交给产品域授权器。
			decision, err := options.Authorizer.AuthorizeRequestScope(ctx, service.RequestScopeAuthorization{
				FullMethod: info.FullMethod,
				Policy:     policy,
				Requested:  requested,
			})
			// 保留授权器返回的 gRPC 状态，避免丢失认证失败或服务不可用语义。
			if err != nil {
				return nil, err
			}
			// 明确拒绝统一返回 PermissionDenied，不删除参数或回退登录上下文。
			if !decision.Allowed {
				// 优先返回产品域提供的安全拒绝原因。
				reason := decision.Reason
				// 授权器没有提供原因时使用稳定的默认消息。
				if reason == "" {
					reason = "request scope is not allowed"
				}
				return nil, status.Error(codes.PermissionDenied, reason)
			}
			// 即使授权器返回允许，也要本地检查有效期和授权集合是否覆盖请求值。
			if err := decision.Validate(requested, now()); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			// 仅把通过二次校验的允许决策写入可信进程内上下文。
			scopeContext.Decision = &decision
		}

		// 把请求作用域上下文交给业务处理器，且不自动传播为下游元数据。
		return handler(service.WithRequestScope(ctx, scopeContext), req)
	}
}
