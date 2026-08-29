package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	// ErrRequestScopePolicyInvalid 表示 RPC 请求作用域策略配置不合法。
	ErrRequestScopePolicyInvalid = errors.New("service: request scope policy is invalid")
	// ErrRequestScopeMessageInvalid 表示已登记 RPC 的请求消息不满足作用域字段约定。
	ErrRequestScopeMessageInvalid = errors.New("service: request scope message is invalid")
	// ErrRequestScopeDecisionInvalid 表示授权器返回了无法覆盖请求范围的错误决策。
	ErrRequestScopeDecisionInvalid = errors.New("service: request scope decision is invalid")
)

// requestScopeContextKey 使用独立类型，避免和其它包写入 context 的字符串键发生冲突。
type requestScopeContextKey string

// requestScopeValueKey 是进程内请求作用域上下文的固定键。
const requestScopeValueKey requestScopeContextKey = "service.request_scope"

// RequestScopeMode 描述业务 RPC 为什么允许调用方显式选择数据作用域。
type RequestScopeMode string

const (
	// RequestScopeModeNone 表示该 RPC 不接受通用请求作用域。
	RequestScopeModeNone RequestScopeMode = "NONE"
	// RequestScopeModeCreate 表示显式作用域用于指定新资源的归属。
	RequestScopeModeCreate RequestScopeMode = "CREATE_OWNER"
	// RequestScopeModeQuery 表示显式作用域用于展开列表或树的查询范围。
	RequestScopeModeQuery RequestScopeMode = "QUERY_RANGE"
	// RequestScopeModeUpdate 表示显式作用域用于修改资源归属信息。
	RequestScopeModeUpdate RequestScopeMode = "UPDATE_OWNER"
	// RequestScopeModeTransfer 表示显式作用域用于执行资源归属迁移。
	RequestScopeModeTransfer RequestScopeMode = "TRANSFER_OWNER"
)

// RequestScopePolicy 是按 RPC 显式登记的请求作用域契约。
// 只有登记后的方法才会把顶层 app_id 或 tenant_id 解释为请求数据作用域。
type RequestScopePolicy struct {
	// Mode 表示当前 RPC 使用请求作用域的业务语义。
	Mode RequestScopeMode
	// AllowAppID 表示当前 RPC 是否允许调用方显式选择应用维度。
	AllowAppID bool
	// AllowTenantID 表示当前 RPC 是否允许调用方显式选择租户维度。
	AllowTenantID bool
}

// Validate 校验单条 RPC 请求作用域策略是否可以安全执行。
func (p RequestScopePolicy) Validate() error {
	// 按模式判断策略是否需要声明可选维度。
	switch p.Mode {
	case RequestScopeModeNone:
		// NONE 模式不会提取任何作用域字段，因此无需声明维度。
		return nil
	case RequestScopeModeCreate, RequestScopeModeQuery, RequestScopeModeUpdate, RequestScopeModeTransfer:
		// 非 NONE 模式至少要允许一个维度，否则登记本身没有可执行意义。
		if !p.AllowAppID && !p.AllowTenantID {
			return fmt.Errorf("%w: mode %s has no allowed dimensions", ErrRequestScopePolicyInvalid, p.Mode)
		}
		// 模式和维度组合有效。
		return nil
	default:
		// 未知模式必须拒绝，避免新增模式在旧服务中被静默放行。
		return fmt.Errorf("%w: unknown mode %q", ErrRequestScopePolicyInvalid, p.Mode)
	}
}

// RequestScopePolicyRegistry 保存按 gRPC FullMethod 登记的不可变策略表。
// 构造完成后不再暴露内部 map，可供多个 gRPC 请求并发只读查询。
type RequestScopePolicyRegistry struct {
	// policies 使用 gRPC FullMethod 作为唯一键。
	policies map[string]RequestScopePolicy
}

// NewRequestScopePolicyRegistry 校验并复制调用方提供的策略表。
func NewRequestScopePolicyRegistry(policies map[string]RequestScopePolicy) (*RequestScopePolicyRegistry, error) {
	// 创建独立 map，防止调用方在构造后修改原 map 影响运行时策略。
	result := &RequestScopePolicyRegistry{policies: make(map[string]RequestScopePolicy, len(policies))}
	// 逐条校验 FullMethod 和策略内容。
	for fullMethod, policy := range policies {
		// 空 FullMethod 无法绑定到具体 RPC，必须在启动期报错。
		if fullMethod == "" {
			return nil, fmt.Errorf("%w: empty full method", ErrRequestScopePolicyInvalid)
		}
		// 策略无效时附带 FullMethod，便于定位具体服务配置。
		if err := policy.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", fullMethod, err)
		}
		// 保存校验后的策略副本。
		result.policies[fullMethod] = policy
	}
	// 返回只读使用的登记表。
	return result, nil
}

// Lookup 按 gRPC FullMethod 查询已登记策略。
func (r *RequestScopePolicyRegistry) Lookup(fullMethod string) (RequestScopePolicy, bool) {
	// nil 登记表或空方法名都按未登记处理。
	if r == nil || fullMethod == "" {
		return RequestScopePolicy{}, false
	}
	// 直接读取构造后不再修改的策略 map。
	policy, ok := r.policies[fullMethod]
	return policy, ok
}

// RequestScope 保存调用方显式提交的请求数据作用域。
// 空字符串按未提供处理，由业务 RPC 使用自身定义的默认作用域。
type RequestScope struct {
	// AppID 是调用方显式选择的应用 ID。
	AppID string
	// TenantID 是调用方显式选择的租户 ID。
	TenantID string
	// AppIDPresent 表示请求中存在非空 app_id。
	AppIDPresent bool
	// TenantIDPresent 表示请求中存在非空 tenant_id。
	TenantIDPresent bool
}

// Explicit 判断本次请求是否显式选择了任一作用域维度。
func (s RequestScope) Explicit() bool {
	// 任一维度存在非空值，就必须进入产品域授权判定。
	return s.AppIDPresent || s.TenantIDPresent
}

// AuthorizedScope 是产品域授权器返回的标准化允许范围。
// 它同时支持离散 ID 集合和明确授予的全应用、全租户范围。
type AuthorizedScope struct {
	// AppIDs 是允许访问的应用 ID 集合。
	AppIDs []string
	// TenantIDs 是允许访问的租户 ID 集合。
	TenantIDs []string
	// AllApplications 表示已明确授予应用维度的全量访问权。
	AllApplications bool
	// AllTenants 表示已明确授予租户维度的全量访问权。
	AllTenants bool
}

// RequestScopeDecision 描述产品域授权器对显式请求作用域作出的决策。
type RequestScopeDecision struct {
	// Allowed 表示是否允许使用本次显式请求作用域。
	Allowed bool
	// Authorized 保存授权器批准的标准化应用和租户范围。
	Authorized AuthorizedScope
	// DecisionID 是本次授权决策的审计标识。
	DecisionID string
	// Reason 保存拒绝原因或安全排障信息。
	Reason string
	// ExpiresAt 是本次决策的失效时间；零值表示框架不额外检查过期时间。
	ExpiresAt time.Time
}

// Validate 对允许决策进行本地二次校验，防止错误授权结果扩大请求范围。
func (d RequestScopeDecision) Validate(requested RequestScope, now time.Time) error {
	// 拒绝决策不携带可执行授权范围，由拦截器直接返回 PermissionDenied。
	if !d.Allowed {
		return nil
	}
	// 非零有效期必须严格晚于当前时间。
	if !d.ExpiresAt.IsZero() && !d.ExpiresAt.After(now) {
		return fmt.Errorf("%w: decision expired", ErrRequestScopeDecisionInvalid)
	}
	// 显式 app_id 必须被全应用授权或应用集合覆盖。
	if requested.AppIDPresent && !d.Authorized.AllApplications && !containsScopeValue(d.Authorized.AppIDs, requested.AppID) {
		return fmt.Errorf("%w: requested app_id is not authorized", ErrRequestScopeDecisionInvalid)
	}
	// 显式 tenant_id 必须被全租户授权或租户集合覆盖。
	if requested.TenantIDPresent && !d.Authorized.AllTenants && !containsScopeValue(d.Authorized.TenantIDs, requested.TenantID) {
		return fmt.Errorf("%w: requested tenant_id is not authorized", ErrRequestScopeDecisionInvalid)
	}
	// 有效期和请求覆盖关系均满足要求。
	return nil
}

// RequestScopeAuthorization 是公共拦截器传给产品域授权器的判定输入。
type RequestScopeAuthorization struct {
	// FullMethod 是当前业务 RPC 的 gRPC 完整方法名。
	FullMethod string
	// Policy 是当前 RPC 在本地登记的作用域语义。
	Policy RequestScopePolicy
	// Requested 是从请求消息中提取出的显式作用域。
	Requested RequestScope
}

// RequestScopeAuthorizer 由 LHDHT authz 等产品域代码实现。
// 实现可以从 ctx 读取已经构建并按需验签的 service.Context。
type RequestScopeAuthorizer interface {
	// AuthorizeRequestScope 判断当前主体是否有权选择请求中的应用和租户范围。
	AuthorizeRequestScope(context.Context, RequestScopeAuthorization) (RequestScopeDecision, error)
}

// RequestScopeContext 是当前进程内可信的请求作用域上下文。
// 该对象不得自动转换为 metadata 或传播到下游服务。
type RequestScopeContext struct {
	// FullMethod 是产生本次作用域上下文的业务 RPC。
	FullMethod string
	// Policy 是该 RPC 使用的本地作用域策略。
	Policy RequestScopePolicy
	// Requested 是调用方显式提交的请求范围。
	Requested RequestScope
	// Decision 是显式作用域对应的授权结果；使用默认作用域时为空。
	Decision *RequestScopeDecision
}

// WithRequestScope 把可信请求作用域写入当前进程的 context。
func WithRequestScope(ctx context.Context, value *RequestScopeContext) context.Context {
	// nil context 或 nil 值不创建新的上下文节点。
	if ctx == nil || value == nil {
		return ctx
	}
	// 使用包内私有键保存作用域对象。
	return context.WithValue(ctx, requestScopeValueKey, value)
}

// RequestScopeFromContext 从当前进程的 context 读取可信请求作用域。
func RequestScopeFromContext(ctx context.Context) (*RequestScopeContext, bool) {
	// nil context 不包含请求作用域。
	if ctx == nil {
		return nil, false
	}
	// 仅接受本包写入的 *RequestScopeContext 类型。
	value, ok := ctx.Value(requestScopeValueKey).(*RequestScopeContext)
	return value, ok
}

// ExtractRequestScope 只提取当前 RPC 策略明确允许的作用域维度。
// 这里使用 protobuf 反射，避免要求每个业务服务实现重复的生成代码适配接口。
func ExtractRequestScope(message proto.Message, policy RequestScopePolicy) (RequestScope, error) {
	// 提取前再次校验策略，防止调用方绕过登记表直接传入无效配置。
	if err := policy.Validate(); err != nil {
		return RequestScope{}, err
	}
	// NONE 模式不读取任何同名业务字段。
	if policy.Mode == RequestScopeModeNone {
		return RequestScope{}, nil
	}
	// 已登记 RPC 必须传入有效的 protobuf 请求消息。
	if message == nil || !message.ProtoReflect().IsValid() {
		return RequestScope{}, ErrRequestScopeMessageInvalid
	}

	// 初始化空作用域；空字段会继续保持未提供状态。
	result := RequestScope{}
	// 只有策略允许应用维度时才读取顶层 app_id。
	if policy.AllowAppID {
		value, present, err := readRequestScopeString(message.ProtoReflect(), "app_id")
		// 字段类型不符合约定时拒绝请求。
		if err != nil {
			return RequestScope{}, err
		}
		// 同时保存字段值和是否显式提供，避免只靠空字符串推断后续状态。
		result.AppID = value
		result.AppIDPresent = present
	}
	// 只有策略允许租户维度时才读取顶层 tenant_id。
	if policy.AllowTenantID {
		value, present, err := readRequestScopeString(message.ProtoReflect(), "tenant_id")
		// 字段类型不符合约定时拒绝请求。
		if err != nil {
			return RequestScope{}, err
		}
		// 同时保存字段值和是否显式提供。
		result.TenantID = value
		result.TenantIDPresent = present
	}
	// 返回按策略提取后的标准化作用域。
	return result, nil
}

// readRequestScopeString 从 protobuf 顶层消息读取指定字符串字段。
func readRequestScopeString(message protoreflect.Message, name protoreflect.Name) (string, bool, error) {
	// 按 protobuf 字段名查找顶层字段描述。
	field := message.Descriptor().Fields().ByName(name)
	// 消息没有该字段时按未提供处理，不解释嵌套对象中的同名字段。
	if field == nil {
		return "", false, nil
	}
	// 请求作用域只接受单值字符串，列表、map 和其它类型都属于契约错误。
	if field.IsList() || field.IsMap() || field.Kind() != protoreflect.StringKind {
		return "", false, fmt.Errorf("%w: field %s must be a string", ErrRequestScopeMessageInvalid, name)
	}
	// 从消息实例读取字段值。
	value := message.Get(field).String()
	// 空字符串表示调用方没有显式选择该维度。
	if value == "" {
		return "", false, nil
	}
	// 非空字符串同时返回值和显式提供标记。
	return value, true, nil
}

// containsScopeValue 判断授权集合是否覆盖指定作用域值。
func containsScopeValue(values []string, target string) bool {
	// 逐项比较标准化 ID；集合规模和缓存策略由产品域授权器控制。
	for _, value := range values {
		// 找到完全相同的授权值时立即返回。
		if value == target {
			return true
		}
	}
	// 集合中不存在目标值。
	return false
}
