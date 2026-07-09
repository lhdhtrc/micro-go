package constant

const (
	// JWSAlgorithmEdDSA 表示 Firefly compact JWS 当前唯一允许的签名算法。
	JWSAlgorithmEdDSA = "EdDSA"
	// JWSTypeJWT 表示 Firefly compact JWS header typ 的标准值。
	JWSTypeJWT = "JWT"
)

const (
	// AuthzSignDefaultKid 是 authz 当前默认使用的 JWS key id。
	AuthzSignDefaultKid = "default"
	// AuthzSignDefaultIssuer 是 x-firefly-authz-sign 当前默认签发方。
	AuthzSignDefaultIssuer = "firefly-authz"
	// AuthzSignDecisionAllow 表示 authz 允许结果的签名取值。
	AuthzSignDecisionAllow = "allow"
)

const (
	// AuthzSignClaimIssuer 是 x-firefly-authz-sign 的 iss claim key。
	AuthzSignClaimIssuer = "iss"
	// AuthzSignClaimSubject 是 x-firefly-authz-sign 的 sub claim key。
	AuthzSignClaimSubject = "sub"
	// AuthzSignClaimSubjectType 是 x-firefly-authz-sign 的 subject_type claim key。
	AuthzSignClaimSubjectType = "subject_type"
	// AuthzSignClaimInvokeAppId 是 x-firefly-authz-sign 的 invoke_app_id claim key。
	AuthzSignClaimInvokeAppId = "invoke_app_id"
	// AuthzSignClaimTargetAppId 是 x-firefly-authz-sign 的 target_app_id claim key。
	AuthzSignClaimTargetAppId = "target_app_id"
	// AuthzSignClaimRouteMethod 是 x-firefly-authz-sign 的 route_method claim key。
	AuthzSignClaimRouteMethod = "route_method"
	// AuthzSignClaimRoutePath 是 x-firefly-authz-sign 的 route_path claim key。
	AuthzSignClaimRoutePath = "route_path"
	// AuthzSignClaimTargetMethod 是 x-firefly-authz-sign 的 target_method claim key。
	AuthzSignClaimTargetMethod = "target_method"
	// AuthzSignClaimTargetPath 是 x-firefly-authz-sign 的 target_path claim key。
	AuthzSignClaimTargetPath = "target_path"
	// AuthzSignClaimUserContext 是 x-firefly-authz-sign 的 user_context claim key。
	AuthzSignClaimUserContext = "user_context"
	// AuthzSignClaimInvokeServiceAppId 是 x-firefly-authz-sign 的 invoke_service_app_id claim key。
	AuthzSignClaimInvokeServiceAppId = "invoke_service_app_id"
	// AuthzSignClaimTargetServiceAppId 是 x-firefly-authz-sign 的 target_service_app_id claim key。
	AuthzSignClaimTargetServiceAppId = "target_service_app_id"
	// AuthzSignClaimDecision 是 x-firefly-authz-sign 的 decision claim key。
	AuthzSignClaimDecision = "decision"
	// AuthzSignClaimDecisionId 是 x-firefly-authz-sign 的 decision_id claim key。
	AuthzSignClaimDecisionId = "decision_id"
	// AuthzSignClaimTraceId 是 x-firefly-authz-sign 的 trace_id claim key。
	AuthzSignClaimTraceId = "trace_id"
	// AuthzSignClaimIssuedAt 是 x-firefly-authz-sign 的 iat claim key。
	AuthzSignClaimIssuedAt = "iat"
	// AuthzSignClaimNotBefore 是 x-firefly-authz-sign 的 nbf claim key。
	AuthzSignClaimNotBefore = "nbf"
	// AuthzSignClaimExpiresAt 是 x-firefly-authz-sign 的 exp claim key。
	AuthzSignClaimExpiresAt = "exp"
)
