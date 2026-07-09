package constant

const (
	// ExtAuthzContextAppId 是 Envoy ext_authz context_extensions 中的 route app_id key。
	ExtAuthzContextAppId = "app_id"
	// ExtAuthzContextRouteMethod 是 Envoy ext_authz context_extensions 中的 route 授权动作 key。
	ExtAuthzContextRouteMethod = "route_method"
	// ExtAuthzContextRoutePath 是 Envoy ext_authz context_extensions 中的 route 授权路径 key。
	ExtAuthzContextRoutePath = "route_path"
	// ExtAuthzContextTargetMethod 是 Envoy ext_authz context_extensions 中的后端目标动作 key。
	ExtAuthzContextTargetMethod = "target_method"
	// ExtAuthzContextTargetPath 是 Envoy ext_authz context_extensions 中的后端目标路径 key。
	ExtAuthzContextTargetPath = "target_path"
	// ExtAuthzContextOrgId 是 Envoy ext_authz context_extensions 中的组织维度 key。
	ExtAuthzContextOrgId = "org_id"
)
