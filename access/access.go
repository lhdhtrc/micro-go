// Package access 提供产品中立的数据访问授权契约。
//
// 本包只负责描述和校验授权决策，以及在当前进程的 context 中保存决策。
// 它不依赖 permission proto、Casbin、GORM、数据库表或 SQL；远程 authz
// 客户端和业务资源的字段/列映射由调用方适配。
package access

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	// ErrDataAccessRequestInvalid 表示授权请求缺少资源或动作。
	ErrDataAccessRequestInvalid = errors.New("access: data access request is invalid")
	// ErrDataAccessDecisionInvalid 表示授权决策无法安全覆盖请求。
	ErrDataAccessDecisionInvalid = errors.New("access: data access decision is invalid")
	// ErrDataAccessDenied 表示资源动作未被授权。
	ErrDataAccessDenied = errors.New("access: data access is denied")
	// ErrDataAccessDecisionExpired 表示授权决策已过期。
	ErrDataAccessDecisionExpired = errors.New("access: data access decision is expired")
	// ErrDataAccessAuthorizerMissing 表示没有装配数据授权器。
	ErrDataAccessAuthorizerMissing = errors.New("access: data access authorizer is missing")
	// ErrDataAccessDecisionMissing 表示 context 中没有所需的授权决策。
	ErrDataAccessDecisionMissing = errors.New("access: data access decision is missing")
	// ErrDataAccessPolicyInvalid 表示 RPC 资源动作登记不合法。
	ErrDataAccessPolicyInvalid = errors.New("access: data access policy is invalid")
)

// ResourceAction 是资源级领域动作的稳定逻辑名称。
// 业务服务可以使用预定义动作，也可以使用非空的领域自定义动作。
type ResourceAction string

const (
	// ResourceActionRead 读取资源。
	ResourceActionRead ResourceAction = "read"
	// ResourceActionCreate 创建资源。
	ResourceActionCreate ResourceAction = "create"
	// ResourceActionUpdate 更新资源。
	ResourceActionUpdate ResourceAction = "update"
	// ResourceActionDelete 删除资源。
	ResourceActionDelete ResourceAction = "delete"
	// ResourceActionExport 导出资源。
	ResourceActionExport ResourceAction = "export"
)

// RequestScope 描述授权请求使用的应用和租户范围。
// 空值表示调用方没有为该维度提交显式范围；具体语义由资源服务决定。
type RequestScope struct {
	// AppID 是访问所属的应用 ID。
	AppID string
	// TenantID 是访问所属的租户 ID。
	TenantID string
}

// Scope 是 RequestScope 的简短别名，便于业务代码构造请求。
type Scope = RequestScope

// DataAccessRequest 是一次资源动作授权请求。
type DataAccessRequest struct {
	// ResourceKey 是跨服务稳定的逻辑资源键，例如 app.application。
	ResourceKey string
	// Action 是资源领域动作。
	Action ResourceAction
	// Scope 是本次请求所属的应用和租户范围。
	Scope RequestScope
}

// Validate 校验授权请求的最小结构，不解释产品域的身份或资源目录。
func (r DataAccessRequest) Validate() error {
	if strings.TrimSpace(r.ResourceKey) == "" {
		return fmt.Errorf("%w: resource key is empty", ErrDataAccessRequestInvalid)
	}
	if strings.TrimSpace(string(r.Action)) == "" {
		return fmt.Errorf("%w: action is empty", ErrDataAccessRequestInvalid)
	}
	return nil
}

// RowConstraintDimension 是结构化行范围的维度编码。
// 数值与 permission 的 ScopeDimension 保持一致，但本包不依赖其 proto 定义。
const (
	ScopeDimensionApplication  uint32 = 1
	ScopeDimensionTenant       uint32 = 2
	ScopeDimensionOrganization uint32 = 3
	ScopeDimensionUser         uint32 = 4
	ScopeDimensionOwner        uint32 = 5
	ScopeDimensionRelation     uint32 = 6
	ScopeDimensionResource     uint32 = 7
	ScopeDimensionAll          uint32 = 8
)

// FieldPermissionAction 是字段级动作编码。
// 数值与 permission 的 FieldPermissionAction 保持一致，避免基础库依赖产品 proto。
const (
	FieldPermissionActionRead   uint32 = 1
	FieldPermissionActionWrite  uint32 = 2
	FieldPermissionActionFilter uint32 = 3
	FieldPermissionActionSort   uint32 = 4
	FieldPermissionActionExport uint32 = 5
)

// RowConstraint 是不含 SQL 的声明式行范围。
// relation_key 只能由资源服务预先登记的 resolver 解释，不能直接当列名或 SQL 片段。
type RowConstraint struct {
	// Dimension 是标准范围维度编码。
	Dimension uint32 `json:"dimension"`
	// Refs 是该维度的参数化引用值集合。
	Refs []string `json:"refs"`
	// IncludeDescendants 表示是否包含组织维度的下级节点。
	IncludeDescendants bool `json:"include_descendants"`
	// RelationKey 是资源服务静态登记的关系范围逻辑键。
	RelationKey string `json:"relation_key"`
}

// DataRowConstraint 保持与数据访问领域命名一致的别名。
type DataRowConstraint = RowConstraint

// FieldActionGrant 描述一个逻辑字段允许执行的字段动作集合。
type FieldActionGrant struct {
	// FieldKey 是业务服务登记的逻辑字段键，不是数据库列名。
	FieldKey string `json:"field_key"`
	// Actions 是字段动作编码集合，例如 read、write、filter、sort、export。
	Actions []uint32 `json:"actions"`
}

// DataFieldActionSet 保持与 authz 数据访问领域命名一致的别名。
type DataFieldActionSet = FieldActionGrant

// FieldActionSet 是一个数据访问决策中的字段动作集合。
type FieldActionSet []FieldActionGrant

// DataAccessDecision 是 authz 返回给业务服务的结构化执行决策。
// 决策只在当前进程内使用，不应写入 JWT、AuthzSign 或普通 metadata。
type DataAccessDecision struct {
	// Allowed 表示资源动作是否允许执行。
	Allowed bool
	// ResourceKey 绑定该决策适用的逻辑资源。
	ResourceKey string
	// Action 绑定该决策适用的资源动作。
	Action ResourceAction
	// RowConstraints 是业务服务需要翻译为参数化查询的范围约束。
	RowConstraints []RowConstraint
	// FieldActions 是业务服务需要执行的逻辑字段动作白名单。
	FieldActions FieldActionSet
	// DecisionID 是本次运行时判定的审计标识。
	DecisionID string
	// ExpiresAt 是决策最晚可执行时间；允许决策必须有明确的未来失效时间。
	ExpiresAt time.Time
}

// Validate 校验决策是否与请求完全绑定、允许执行且尚未过期。
// now 为零值时使用当前 UTC 时间，便于生产调用和测试注入固定时钟。
func (d DataAccessDecision) Validate(request DataAccessRequest, now time.Time) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(d.ResourceKey) == "" || strings.TrimSpace(string(d.Action)) == "" {
		return fmt.Errorf("%w: resource key or action is empty", ErrDataAccessDecisionInvalid)
	}
	if d.ResourceKey != request.ResourceKey || d.Action != request.Action {
		return fmt.Errorf("%w: decision does not match request", ErrDataAccessDecisionInvalid)
	}
	if !d.Allowed {
		return ErrDataAccessDenied
	}
	if d.ExpiresAt.IsZero() {
		return fmt.Errorf("%w: expires_at is missing", ErrDataAccessDecisionInvalid)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !d.ExpiresAt.After(now) {
		return ErrDataAccessDecisionExpired
	}
	if err := validateRowConstraints(d.RowConstraints); err != nil {
		return err
	}
	if err := validateFieldActions(d.FieldActions); err != nil {
		return err
	}
	return nil
}

// IsExpired 判断决策是否已经失效；缺少失效时间也按失效处理。
func (d DataAccessDecision) IsExpired(now time.Time) bool {
	if d.ExpiresAt.IsZero() {
		return true
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !d.ExpiresAt.After(now)
}

// DataAccessAuthorizer 是业务服务对 authz 适配后的中立授权接口。
type DataAccessAuthorizer interface {
	// Authorize 返回绑定到请求资源和动作的结构化决策，不返回 SQL。
	Authorize(context.Context, DataAccessRequest) (DataAccessDecision, error)
}

// DataAccessAuthorizerFunc 将普通函数适配为 DataAccessAuthorizer。
type DataAccessAuthorizerFunc func(context.Context, DataAccessRequest) (DataAccessDecision, error)

// Authorize 调用授权器并在返回前执行本地 fail-close 校验。
func Authorize(ctx context.Context, authorizer DataAccessAuthorizer, request DataAccessRequest) (DataAccessDecision, error) {
	if authorizer == nil {
		return DataAccessDecision{}, ErrDataAccessAuthorizerMissing
	}
	if err := request.Validate(); err != nil {
		return DataAccessDecision{}, err
	}
	decision, err := authorizer.Authorize(ctx, request)
	if err != nil {
		return DataAccessDecision{}, err
	}
	if err := decision.Validate(request, time.Now().UTC()); err != nil {
		return decision, err
	}
	return cloneDecision(decision), nil
}

// AuthorizeDataAccess 是 Authorize 的语义化别名，方便业务代码按 RPC 名称调用。
func AuthorizeDataAccess(ctx context.Context, authorizer DataAccessAuthorizer, request DataAccessRequest) (DataAccessDecision, error) {
	return Authorize(ctx, authorizer, request)
}

// DataAccessAuthorizerFunc 实现 DataAccessAuthorizer。
func (f DataAccessAuthorizerFunc) Authorize(ctx context.Context, request DataAccessRequest) (DataAccessDecision, error) {
	if f == nil {
		return DataAccessDecision{}, ErrDataAccessAuthorizerMissing
	}
	return f(ctx, request)
}

// decisionContextValue 保存一组按资源动作索引的不可变决策。
type decisionContextValue struct {
	decisions map[string]DataAccessDecision
}

type decisionContextKey struct{}

// WithDataAccessDecision 将决策写入当前进程 context，并复制所有切片。
// 已存在的同资源同动作决策会被新的决策覆盖，其他决策保持不变。
func WithDataAccessDecision(ctx context.Context, decision *DataAccessDecision) context.Context {
	if ctx == nil || decision == nil {
		return ctx
	}
	value := &decisionContextValue{decisions: make(map[string]DataAccessDecision)}
	if previous, ok := ctx.Value(decisionContextKey{}).(*decisionContextValue); ok && previous != nil {
		for key, item := range previous.decisions {
			value.decisions[key] = cloneDecision(item)
		}
	}
	value.decisions[DecisionKey(decision.ResourceKey, decision.Action)] = cloneDecision(*decision)
	return context.WithValue(ctx, decisionContextKey{}, value)
}

// DataAccessDecisionFromContext 读取指定资源动作的进程内决策副本。
func DataAccessDecisionFromContext(ctx context.Context, resourceKey string, action ResourceAction) (*DataAccessDecision, bool) {
	if ctx == nil {
		return nil, false
	}
	value, ok := ctx.Value(decisionContextKey{}).(*decisionContextValue)
	if !ok || value == nil {
		return nil, false
	}
	decision, ok := value.decisions[DecisionKey(resourceKey, action)]
	if !ok {
		return nil, false
	}
	copy := cloneDecision(decision)
	return &copy, true
}

// MustDataAccessDecisionFromContext 读取决策，缺失时返回 ErrDataAccessDecisionMissing。
func MustDataAccessDecisionFromContext(ctx context.Context, resourceKey string, action ResourceAction) (DataAccessDecision, error) {
	decision, ok := DataAccessDecisionFromContext(ctx, resourceKey, action)
	if !ok || decision == nil {
		return DataAccessDecision{}, ErrDataAccessDecisionMissing
	}
	return *decision, nil
}

// DecisionKey 返回 context registry 使用的稳定资源动作键。
func DecisionKey(resourceKey string, action ResourceAction) string {
	return strings.TrimSpace(resourceKey) + "\x00" + strings.TrimSpace(string(action))
}

// DataAccessPolicy 描述一个 RPC 预检所需的逻辑资源动作。
type DataAccessPolicy struct {
	// ResourceKey 是跨服务稳定的逻辑资源键。
	ResourceKey string
	// Action 是该 RPC 需要预检的资源动作。
	Action ResourceAction
}

// DataAccessPolicyRegistry 保存按 gRPC FullMethod 登记的不可变资源动作表。
type DataAccessPolicyRegistry struct {
	policies map[string][]DataAccessPolicy
}

// NewDataAccessPolicyRegistry 校验并复制策略表，构造后可安全并发读取。
func NewDataAccessPolicyRegistry(policies map[string][]DataAccessPolicy) (*DataAccessPolicyRegistry, error) {
	result := &DataAccessPolicyRegistry{policies: make(map[string][]DataAccessPolicy, len(policies))}
	for fullMethod, entries := range policies {
		fullMethod = strings.TrimSpace(fullMethod)
		if fullMethod == "" || len(entries) == 0 {
			return nil, fmt.Errorf("%w: method and entries are required", ErrDataAccessPolicyInvalid)
		}
		seen := make(map[string]struct{}, len(entries))
		copied := make([]DataAccessPolicy, 0, len(entries))
		for _, entry := range entries {
			entry.ResourceKey = strings.TrimSpace(entry.ResourceKey)
			entry.Action = ResourceAction(strings.TrimSpace(string(entry.Action)))
			if entry.ResourceKey == "" || entry.Action == "" {
				return nil, fmt.Errorf("%w: method %s contains empty resource or action", ErrDataAccessPolicyInvalid, fullMethod)
			}
			key := DecisionKey(entry.ResourceKey, entry.Action)
			if _, duplicate := seen[key]; duplicate {
				return nil, fmt.Errorf("%w: method %s contains duplicate resource action", ErrDataAccessPolicyInvalid, fullMethod)
			}
			seen[key] = struct{}{}
			copied = append(copied, entry)
		}
		sort.Slice(copied, func(i, j int) bool {
			if copied[i].ResourceKey == copied[j].ResourceKey {
				return copied[i].Action < copied[j].Action
			}
			return copied[i].ResourceKey < copied[j].ResourceKey
		})
		result.policies[fullMethod] = copied
	}
	return result, nil
}

// Lookup 返回指定 FullMethod 的策略副本；未登记时返回 false。
func (r *DataAccessPolicyRegistry) Lookup(fullMethod string) ([]DataAccessPolicy, bool) {
	if r == nil {
		return nil, false
	}
	entries, ok := r.policies[strings.TrimSpace(fullMethod)]
	if !ok {
		return nil, false
	}
	return append([]DataAccessPolicy(nil), entries...), true
}

func validateRowConstraints(constraints []RowConstraint) error {
	for index, constraint := range constraints {
		if constraint.Dimension == 0 {
			return fmt.Errorf("%w: row constraint %d has unspecified dimension", ErrDataAccessDecisionInvalid, index)
		}
		seen := make(map[string]struct{}, len(constraint.Refs))
		for _, ref := range constraint.Refs {
			ref = strings.TrimSpace(ref)
			if ref == "" {
				return fmt.Errorf("%w: row constraint %d has empty ref", ErrDataAccessDecisionInvalid, index)
			}
			if _, duplicate := seen[ref]; duplicate {
				return fmt.Errorf("%w: row constraint %d has duplicate ref", ErrDataAccessDecisionInvalid, index)
			}
			seen[ref] = struct{}{}
		}
		if constraint.Dimension == ScopeDimensionAll && len(constraint.Refs) != 0 {
			return fmt.Errorf("%w: all dimension cannot contain refs", ErrDataAccessDecisionInvalid)
		}
		if constraint.Dimension == ScopeDimensionRelation && strings.TrimSpace(constraint.RelationKey) == "" {
			return fmt.Errorf("%w: relation dimension requires relation key", ErrDataAccessDecisionInvalid)
		}
		if constraint.Dimension != ScopeDimensionRelation && strings.TrimSpace(constraint.RelationKey) != "" {
			return fmt.Errorf("%w: relation key is only valid for relation dimension", ErrDataAccessDecisionInvalid)
		}
		if constraint.IncludeDescendants && constraint.Dimension != ScopeDimensionOrganization {
			return fmt.Errorf("%w: descendants are only valid for organization dimension", ErrDataAccessDecisionInvalid)
		}
	}
	return nil
}

func validateFieldActions(fields FieldActionSet) error {
	for index, field := range fields {
		if strings.TrimSpace(field.FieldKey) == "" || len(field.Actions) == 0 {
			return fmt.Errorf("%w: field action %d is incomplete", ErrDataAccessDecisionInvalid, index)
		}
		seen := make(map[uint32]struct{}, len(field.Actions))
		for _, action := range field.Actions {
			if action == 0 {
				return fmt.Errorf("%w: field action %d contains unspecified action", ErrDataAccessDecisionInvalid, index)
			}
			if _, duplicate := seen[action]; duplicate {
				return fmt.Errorf("%w: field action %d contains duplicate action", ErrDataAccessDecisionInvalid, index)
			}
			seen[action] = struct{}{}
		}
	}
	return nil
}

func cloneDecision(source DataAccessDecision) DataAccessDecision {
	source.RowConstraints = append([]RowConstraint(nil), source.RowConstraints...)
	for index := range source.RowConstraints {
		source.RowConstraints[index].Refs = append([]string(nil), source.RowConstraints[index].Refs...)
	}
	source.FieldActions = append(FieldActionSet(nil), source.FieldActions...)
	for index := range source.FieldActions {
		source.FieldActions[index].Actions = append([]uint32(nil), source.FieldActions[index].Actions...)
	}
	return source
}
