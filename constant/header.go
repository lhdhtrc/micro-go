// Package constant 定义 Firefly 微服务通信中稳定使用的 HTTP Header / gRPC Metadata key。
package constant

const (
	// XRealIp 表示入口代理透传的真实客户端 IP，通常由 Nginx / Ingress 写入。
	XRealIp = "x-real-ip"
	// XForwardedFor 表示标准代理链路 IP 列表，authz 可读取第一个地址作为原始客户端 IP。
	XForwardedFor = "x-forwarded-for"
	// TraceParent 是 W3C Trace Context 中承载 trace_id/span_id/parent 关系的标准头。
	TraceParent = "traceparent"
	// TraceState 是 W3C Trace Context 中承载厂商扩展 trace 状态的标准头。
	TraceState = "tracestate"
	// Baggage 是 W3C Baggage 标准头，用于跨进程传播低基数业务上下文。
	Baggage = "baggage"

	// HeaderPrefix 是 Firefly 自定义 header 的统一前缀。
	HeaderPrefix = "x-firefly-"
)

const (
	// AppLanguage 表示客户端应用语言偏好，可作为访问日志和业务展示上下文使用。
	AppLanguage = HeaderPrefix + "app-language"
	// AppVersion 表示客户端应用版本。
	AppVersion = HeaderPrefix + "app-version"
)

const (
	// SubjectType 表示 authz 注入的普通主体类型，取值见 SubjectTypeAnonymous/User/Service。
	SubjectType = HeaderPrefix + "subject-type"
	// DecisionId 表示 authz 对本次 allow 判定生成的普通决策 ID，用于日志关联。
	DecisionId = HeaderPrefix + "decision-id"
	// AuthzSign 表示 authz 写入的短有效期 compact JWS，是服务侧本地验签输入。
	AuthzSign = HeaderPrefix + "authz-sign"
)

const (
	// SystemType 表示客户端操作系统类型枚举值。
	SystemType = HeaderPrefix + "system-type"
	// SystemName 表示客户端操作系统名称。
	SystemName = HeaderPrefix + "system-name"
	// SystemVersion 表示客户端操作系统版本。
	SystemVersion = HeaderPrefix + "system-version"
	// ClientType 表示客户端类型枚举值。
	ClientType = HeaderPrefix + "client-type"
	// ClientName 表示客户端名称。
	ClientName = HeaderPrefix + "client-name"
	// ClientVersion 表示客户端版本。
	ClientVersion = HeaderPrefix + "client-version"
)

const (
	// UserAuthority 表示跨进程传输的用户 authority 原文，只由 authz 负责校验和解析。
	UserAuthority = HeaderPrefix + "user-authority"
	// ServiceAuthority 表示跨进程传输的服务 authority 原文，每一跳由当前调用服务覆盖写入。
	ServiceAuthority = HeaderPrefix + "service-authority"
)

const (
	// ServiceAppId 表示当前业务服务自身的应用 ID，只由本服务入口注入到本地上下文。
	//
	// 它不参与 authz 权限元组，也不允许跨服务出站透传。
	ServiceAppId = HeaderPrefix + "service-app-id"
	// ServiceInstanceId 表示当前业务服务自身的实例 ID，只由本服务入口注入到本地上下文。
	//
	// 它用于日志、OTel 和数据库链路排障，不参与 authz 权限元组，也不允许跨服务出站透传。
	ServiceInstanceId = HeaderPrefix + "service-instance-id"
)

// 以下字段由 authz 从 X-Firefly-User-Authority 中解析后注入，业务侧按 UserContext 读取。
const (
	// UserId 表示用户主体 ID，服务主体或匿名主体为空。
	UserId = HeaderPrefix + "user-id"
	// Session 表示用户 token 关联的会话标识。
	Session = HeaderPrefix + "session"
	// OrgIds 表示用户关联的组织 ID 列表。
	OrgIds = HeaderPrefix + "org-ids"
	// PostIds 表示用户关联的岗位 ID 列表。
	PostIds = HeaderPrefix + "post-ids"
	// RoleIds 表示用户关联的角色 ID 列表。
	RoleIds = HeaderPrefix + "role-ids"
	// AppId 表示用户身份中的应用 ID，不表达服务调用方。
	AppId = HeaderPrefix + "app-id"
	// TenantId 表示用户所属租户 ID。
	TenantId = HeaderPrefix + "tenant-id"
)

const (
	// InvokeAppId 表示 authz 本次授权元组中的调用方 app_id。
	//
	// 用户入口请求没有服务身份时可来自 UserContext.app_id；
	// 服务间调用时来自 InvokeServiceAppId。
	InvokeAppId = HeaderPrefix + "invoke-app-id"
	// TargetAppId 表示 authz 从 route.app_id 映射出的被访问服务 app_id。
	TargetAppId = HeaderPrefix + "target-app-id"
)

const (
	// RouteMethod 表示当前授权检查命中的 route 动作，例如 GET/POST/GRPC。
	RouteMethod = HeaderPrefix + "route-method"
	// RoutePath 表示当前授权检查命中的 route 路径，HTTP 为 path，gRPC 为 FullMethod。
	RoutePath = HeaderPrefix + "route-path"
	// TargetMethod 表示授权结果最终投递到后端服务的目标动作。
	TargetMethod = HeaderPrefix + "target-method"
	// TargetPath 表示授权结果最终投递到后端服务的目标资源。
	TargetPath = HeaderPrefix + "target-path"
)
