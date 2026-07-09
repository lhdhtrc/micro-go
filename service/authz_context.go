package service

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/fireflycore/go-micro/constant"
)

var (
	// ErrAuthzSignMissing 表示入口请求没有携带 authz 注入的 compact JWS。
	ErrAuthzSignMissing = errors.New("authz sign is missing")
	// ErrAuthzSignMalformed 表示输入不是合法的 compact JWS。
	ErrAuthzSignMalformed = errors.New("authz sign is malformed")
	// ErrAuthzSignPublicKeyMissing 表示无法根据 kid 或默认配置找到验签公钥。
	ErrAuthzSignPublicKeyMissing = errors.New("authz sign public key is missing")
	// ErrAuthzSignUnsupportedAlg 表示 JWS alg 不是当前约定的签名算法。
	ErrAuthzSignUnsupportedAlg = errors.New("authz sign alg is unsupported")
	// ErrAuthzSignInvalidSignature 表示 JWS 签名验证失败。
	ErrAuthzSignInvalidSignature = errors.New("authz sign signature is invalid")
	// ErrAuthzSignInvalidClaims 表示 JWS claim 缺少必要字段或与本地期望不一致。
	ErrAuthzSignInvalidClaims = errors.New("authz sign claims are invalid")
	// ErrAuthzSignExpired 表示 JWS 已过期。
	ErrAuthzSignExpired = errors.New("authz sign is expired")
	// ErrAuthzSignNotYetValid 表示 JWS 尚未到可用时间。
	ErrAuthzSignNotYetValid = errors.New("authz sign is not yet valid")
)

// AuthzSign 表示 x-firefly-authz-sign compact JWS 验签通过后的 payload。
//
// 它不是传输层 metadata；跨进程传输形式始终是 constant.AuthzSign 对应的 compact JWS。
type AuthzSign struct {
	// KeyId 是 JWS header 中的 kid，用于密钥轮换排查。
	KeyId string `json:"-"`
	// Issuer 表示签发方，通常由业务服务配置指定期望值。
	Issuer string `json:"iss"`
	// SubjectId 表示进入 Casbin 的主体 ID，当前通常等于 invoke_app_id 或 anonymous。
	SubjectId string `json:"sub"`
	// SubjectType 表示主体类型，取值见 constant.SubjectTypeAnonymous/User/Service。
	SubjectType string `json:"subject_type"`
	// UserId 表示用户主体 ID，来自 user_context.user_id；该字段由验签后派生，不从 JSON 平铺字段读取。
	UserId string `json:"-"`
	// AppId 表示用户身份中的应用 ID，来自 user_context.app_id；该字段由验签后派生。
	AppId string `json:"-"`
	// Session 表示 authz 从 token 中解析出的会话标识，来自 user_context.session。
	Session string `json:"-"`
	// TenantId 表示用户所属租户 ID，来自 user_context.tenant_id。
	TenantId string `json:"-"`
	// OrgIds 表示用户关联的组织 ID 列表，来自 user_context.org_ids。
	OrgIds []string `json:"-"`
	// PostIds 表示用户关联的岗位 ID 列表，来自 user_context.post_ids。
	PostIds []string `json:"-"`
	// RoleIds 表示用户关联的角色 ID 列表，来自 user_context.role_ids。
	RoleIds []string `json:"-"`
	// InvokeAppId 表示本跳权限判定中的调用方应用 ID，来自 invoke_app_id。
	InvokeAppId string `json:"invoke_app_id"`
	// TargetAppId 表示 authz 对 route.app_id 的判定语义，来自 target_app_id。
	TargetAppId string `json:"target_app_id"`
	// RouteMethod 表示本次授权动作，取值见 constant.RequestMethod*String。
	RouteMethod string `json:"route_method"`
	// RoutePath 表示本次授权资源路径，HTTP 为入口 path，gRPC 为 FullMethod。
	RoutePath string `json:"route_path"`
	// TargetMethod 表示本次授权结果最终投递到后端服务的目标动作。
	TargetMethod string `json:"target_method,omitempty"`
	// TargetPath 表示本次授权结果最终投递到后端服务的目标资源。
	TargetPath string `json:"target_path,omitempty"`
	// Decision 表示 authz 判定结果，当前只接受 allow。
	Decision string `json:"decision"`
	// DecisionId 表示 authz 对本次判定生成的唯一 ID。
	DecisionId string `json:"decision_id"`
	// TraceId 表示 authz 从 traceparent 中提取的 OTel trace_id。
	TraceId string `json:"trace_id,omitempty"`
	// UserContext 保存 authz 从用户 authority 还原出的用户身份上下文。
	UserContext *AuthzUserContext `json:"user_context,omitempty"`
	// InvokeServiceAppId 保存 authz 从服务 authority 还原出的当前跳调用服务 app_id。
	InvokeServiceAppId string `json:"invoke_service_app_id,omitempty"`
	// TargetServiceAppId 保存 authz 从 route 事实映射出的被访问服务 app_id。
	TargetServiceAppId string `json:"target_service_app_id"`
	// IssuedAt 表示签发时间，Unix 秒。
	IssuedAt int64 `json:"iat"`
	// NotBefore 表示最早可用时间，Unix 秒；当前 authz 可不写。
	NotBefore int64 `json:"nbf,omitempty"`
	// ExpiresAt 表示过期时间，Unix 秒。
	ExpiresAt int64 `json:"exp"`
}

// AuthzUserContext 表示 JWS payload 中 user_context 的结构化用户身份。
type AuthzUserContext struct {
	// UserId 是用户主体 ID。
	UserId string `json:"user_id,omitempty"`
	// AppId 是用户身份中的应用 ID，不等同于本跳服务调用方 app_id。
	AppId string `json:"app_id,omitempty"`
	// TenantId 是用户所属租户 ID。
	TenantId string `json:"tenant_id,omitempty"`
	// Session 是用户 token 关联的会话标识。
	Session string `json:"session,omitempty"`
	// OrgIds 是用户关联组织 ID 列表。
	OrgIds []string `json:"org_ids,omitempty"`
	// PostIds 是用户关联岗位 ID 列表。
	PostIds []string `json:"post_ids,omitempty"`
	// RoleIds 是用户关联角色 ID 列表。
	RoleIds []string `json:"role_ids,omitempty"`
}

// AuthzSignVerificationOptions 定义服务侧本地验签 x-firefly-authz-sign 的规则。
type AuthzSignVerificationOptions struct {
	// PublicKey 是单公钥模式下的 Ed25519 公钥。
	PublicKey ed25519.PublicKey
	// PublicKeys 是按 kid 索引的 Ed25519 公钥集合，用于密钥轮换。
	PublicKeys map[string]ed25519.PublicKey
	// Issuer 非空时要求 iss 必须与该值一致。
	Issuer string
	// ExpectedRouteMethod 非空时要求 route_method 必须匹配当前入口授权动作。
	ExpectedRouteMethod string
	// ExpectedRoutePath 非空时要求 route_path 必须匹配当前入口资源路径。
	ExpectedRoutePath string
	// ExpectedTargetMethod 非空时要求 target_method 必须匹配当前服务入口目标动作。
	ExpectedTargetMethod string
	// ExpectedTargetPath 非空时要求 target_path 必须匹配当前服务入口目标资源。
	ExpectedTargetPath string
	// ClockSkew 表示允许的本机时钟偏差。
	ClockSkew time.Duration
	// Now 允许测试注入固定时间；生产环境留空使用 time.Now。
	Now func() time.Time
}

type authzSignJWSHeader struct {
	// Alg 表示 JWS 签名算法，当前只接受 constant.JWSAlgorithmEdDSA。
	Alg string `json:"alg"`
	// Kid 表示签名密钥 ID，用于从 PublicKeys 中选择公钥。
	Kid string `json:"kid"`
	// Typ 表示 token 类型，通常为 constant.JWSTypeJWT；当前不参与判定。
	Typ string `json:"typ"`
}

// VerifyAuthzSign 校验 authz compact JWS 并返回可信 payload。
func VerifyAuthzSign(raw string, options AuthzSignVerificationOptions) (*AuthzSign, error) {
	// 先裁剪外部传入值，避免 header 前后空白影响 compact JWS 解析。
	raw = strings.TrimSpace(raw)
	// 没有 JWS 时直接返回明确错误，由入口决定是否允许跳过。
	if raw == "" {
		return nil, ErrAuthzSignMissing
	}

	// compact JWS 固定由 header.payload.signature 三段组成。
	parts := strings.Split(raw, ".")
	// 任意一段缺失都说明不是合法 JWS，直接拒绝。
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, ErrAuthzSignMalformed
	}

	// 先解析 header，拿到 alg 和 kid 后才能选择验签策略。
	header, err := parseAuthzSignHeader(parts[0])
	if err != nil {
		return nil, err
	}
	// 当前 authz 约定使用 Ed25519/EdDSA，拒绝 alg 降级或替换。
	if header.Alg != constant.JWSAlgorithmEdDSA {
		return nil, ErrAuthzSignUnsupportedAlg
	}

	// 根据 kid 选择公钥；未配置多公钥时退化为单公钥模式。
	publicKey, ok := resolveAuthzSignPublicKey(header.Kid, options)
	if !ok {
		return nil, ErrAuthzSignPublicKeyMissing
	}

	// 签名段必须是 base64url，无 padding。
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrAuthzSignMalformed
	}
	// Ed25519 的签名输入必须是原始 header.payload 字符串，不能用解码后的 JSON 重组。
	if !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return nil, ErrAuthzSignInvalidSignature
	}

	// 签名通过后再解码 payload，避免在未可信数据上做多余业务处理。
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrAuthzSignMalformed
	}

	// 先把 kid 写入上下文，便于日志或排障时知道使用的是哪把公钥。
	claims := &AuthzSign{KeyId: header.Kid}
	// 将 JSON claim 反序列化为稳定结构，避免业务侧直接操作 map。
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, ErrAuthzSignMalformed
	}
	// 只从 current 目标 claim 派生进程内读取便利字段，平铺身份 claim 不进入 current 语义。
	normalizeVerifiedAuthzSign(claims)
	// 最后校验 issuer、时间窗口和当前入口期望的方法/路径事实。
	if err := validateAuthzSignClaims(claims, options); err != nil {
		return nil, err
	}
	// 返回已经验签和校验过的可信 payload。
	return claims, nil
}

func parseAuthzSignHeader(segment string) (*authzSignJWSHeader, error) {
	// JWS header 使用 base64url 编码。
	data, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		return nil, ErrAuthzSignMalformed
	}
	// 解析出 alg/kid/typ，后续用 alg 和 kid 做验签约束。
	header := &authzSignJWSHeader{}
	if err := json.Unmarshal(data, header); err != nil {
		return nil, ErrAuthzSignMalformed
	}
	// 返回原始 header 结构，不在这里做业务校验。
	return header, nil
}

func resolveAuthzSignPublicKey(kid string, options AuthzSignVerificationOptions) (ed25519.PublicKey, bool) {
	// 多公钥模式优先，便于 authz 密钥轮换期间同时接受新旧 kid。
	if len(options.PublicKeys) > 0 {
		// 通过 JWS header 中的 kid 找到对应公钥。
		key, ok := options.PublicKeys[kid]
		// Ed25519 公钥必须是固定长度，否则视为配置错误。
		return key, ok && len(key) == ed25519.PublicKeySize
	}
	// 单公钥模式用于简单部署或测试环境。
	if len(options.PublicKey) == ed25519.PublicKeySize {
		return options.PublicKey, true
	}
	// 没有可用公钥时返回 false，由上层转换成明确错误。
	return nil, false
}

func validateAuthzSignClaims(claims *AuthzSign, options AuthzSignVerificationOptions) error {
	// claim 为空说明 payload 没有成功构造成可信上下文。
	if claims == nil {
		return ErrAuthzSignInvalidClaims
	}
	// 配置了 issuer 时必须严格匹配，防止其他签发方 token 混入。
	if options.Issuer != "" && claims.Issuer != options.Issuer {
		return ErrAuthzSignInvalidClaims
	}
	// 主体、目标应用、授权动作和资源路径是服务侧验签后的最小可信字段。
	if claims.SubjectId == "" || claims.SubjectType == "" || claims.TargetAppId == "" || claims.RouteMethod == "" || claims.RoutePath == "" {
		return ErrAuthzSignInvalidClaims
	}
	// 非匿名请求必须有调用方 app_id；公共接口允许 invoke_app_id 为空。
	if claims.SubjectType != constant.SubjectTypeAnonymous && claims.InvokeAppId == "" {
		return ErrAuthzSignInvalidClaims
	}
	// 用户主体必须携带结构化 UserContext，避免旧平铺 claim 被继续接受。
	if claims.SubjectType == constant.SubjectTypeUser && (claims.UserContext == nil || claims.UserContext.UserId == "" || claims.UserContext.AppId == "") {
		return ErrAuthzSignInvalidClaims
	}
	// 服务主体必须携带服务 authority 解析出的 app_id，不能只靠 invoke_app_id 反推服务身份。
	if claims.SubjectType == constant.SubjectTypeService && claims.InvokeServiceAppId == "" {
		return ErrAuthzSignInvalidClaims
	}
	// 所有允许进入业务服务的请求都必须携带 route 解析出的服务 app_id。
	if claims.TargetServiceAppId == "" {
		return ErrAuthzSignInvalidClaims
	}
	// 平铺 target_app_id 必须与 route.app_id 映射结果一致。
	if claims.TargetAppId != claims.TargetServiceAppId {
		return ErrAuthzSignInvalidClaims
	}
	// 服务主体的平铺 invoke_app_id 必须与服务 authority 解析结果一致。
	if claims.SubjectType == constant.SubjectTypeService && claims.InvokeAppId != claims.InvokeServiceAppId {
		return ErrAuthzSignInvalidClaims
	}
	// 用户主体的权限主体必须来自 UserContext.app_id，即使入口同时携带 service authority 也不能覆盖。
	if claims.SubjectType == constant.SubjectTypeUser && claims.InvokeAppId != claims.UserContext.AppId {
		return ErrAuthzSignInvalidClaims
	}
	// 当前只有 allow 结果会被 authz 注入业务服务，缺失 decision 或非 allow 都不能进入业务层。
	if claims.Decision != constant.AuthzSignDecisionAllow {
		return ErrAuthzSignInvalidClaims
	}
	// target_method/target_path 是服务侧绑定校验字段；要么都不写，要么成对出现。
	if (claims.TargetMethod == "") != (claims.TargetPath == "") {
		return ErrAuthzSignInvalidClaims
	}

	// 默认使用系统时间，测试可通过 options.Now 注入固定时间。
	now := time.Now
	if options.Now != nil {
		now = options.Now
	}
	// 统一转 UTC，避免本地时区影响 Unix 秒比较。
	current := now().UTC()
	// ClockSkew 用于容忍极小范围的机器时钟偏差。
	skew := options.ClockSkew
	// exp 是必需字段；当前时间超过 exp 后 token 失效。
	if claims.ExpiresAt <= 0 || !current.Add(-skew).Before(time.Unix(claims.ExpiresAt, 0)) {
		return ErrAuthzSignExpired
	}
	// nbf 可选；写入时表示当前时间必须晚于 nbf。
	if claims.NotBefore > 0 && current.Add(skew).Before(time.Unix(claims.NotBefore, 0)) {
		return ErrAuthzSignNotYetValid
	}
	// 授权动作非空时必须匹配当前入口；gRPC 场景通常是 GRPC。
	if options.ExpectedRouteMethod != "" && !strings.EqualFold(claims.RouteMethod, options.ExpectedRouteMethod) {
		return ErrAuthzSignInvalidClaims
	}
	// 资源路径非空时必须匹配当前 FullMethod 或 HTTP path。
	if options.ExpectedRoutePath != "" && claims.RoutePath != options.ExpectedRoutePath {
		return ErrAuthzSignInvalidClaims
	}
	targetMethod, targetPath := effectiveAuthzTarget(claims)
	if options.ExpectedTargetMethod != "" {
		if targetMethod == "" || !strings.EqualFold(targetMethod, options.ExpectedTargetMethod) {
			return ErrAuthzSignInvalidClaims
		}
	}
	if options.ExpectedTargetPath != "" {
		if targetPath == "" || targetPath != options.ExpectedTargetPath {
			return ErrAuthzSignInvalidClaims
		}
	}
	// 所有必要 claim 和本地期望都通过后，认为上下文可信。
	return nil
}

func effectiveAuthzTarget(claims *AuthzSign) (string, string) {
	// 新版 AuthzSign 使用 target_method/target_path 绑定后端目标。
	if claims == nil {
		return "", ""
	}
	if claims.TargetMethod != "" || claims.TargetPath != "" {
		return claims.TargetMethod, claims.TargetPath
	}
	// 兼容未显式写 target 的原生 gRPC sign：route_* 本身就是后端 gRPC 目标。
	if strings.EqualFold(claims.RouteMethod, constant.RequestMethodGrpcString) {
		return claims.RouteMethod, claims.RoutePath
	}
	return "", ""
}

func normalizeVerifiedAuthzSign(claims *AuthzSign) {
	// 空 payload 无需处理，调用方会在 validateAuthzSignClaims 中拒绝。
	if claims == nil {
		return
	}
	// user_context 是目标结构；UserId/AppId 等扁平字段只作为验签后的进程内读取便利。
	if claims.UserContext != nil {
		claims.UserId = claims.UserContext.UserId
		claims.AppId = claims.UserContext.AppId
		claims.TenantId = claims.UserContext.TenantId
		claims.Session = claims.UserContext.Session
		claims.OrgIds = cloneStrings(claims.UserContext.OrgIds)
		claims.PostIds = cloneStrings(claims.UserContext.PostIds)
		claims.RoleIds = cloneStrings(claims.UserContext.RoleIds)
	}
	// 服务鉴权当前只使用 app_id 粒度，invoke/target instance 不进入 AuthzSign 运行时字段。
}
