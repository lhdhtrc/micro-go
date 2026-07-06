package authz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultServiceAuthorityRefreshBefore 是 service token 过期前的默认主动刷新窗口。
	DefaultServiceAuthorityRefreshBefore = time.Minute
	// DefaultServiceAuthorityRetryBaseInterval 是后台获取 service token 失败后的基础退避间隔。
	DefaultServiceAuthorityRetryBaseInterval = time.Minute
	// DefaultServiceAuthorityRetryMaxInterval 是后台获取 service token 失败后的最大退避间隔。
	DefaultServiceAuthorityRetryMaxInterval = time.Hour
	// serviceAuthorityRetryMultiplier 固定表达 1 分钟 * 重试次数 * 10 的业务退避规则。
	serviceAuthorityRetryMultiplier = 10
)

var (
	// ErrServiceAuthorityFetchMissing 表示启用 service authority 时没有配置取 token 函数。
	ErrServiceAuthorityFetchMissing = errors.New("authz service authority fetch function is missing")
	// ErrServiceTokenUnavailable 表示当前进程还没有可用于发起下游 Firefly 调用的 service token。
	ErrServiceTokenUnavailable = errors.New("authz service token is unavailable")
	// ErrServiceAuthorityTokenMissing 表示取到的 service token 为空，不能写入出站请求。
	ErrServiceAuthorityTokenMissing = errors.New("authz service authority token is missing")
	// ErrServiceAuthorityTokenExpiresAtMissing 表示 service token 没有明确过期时间，无法做动态轮换。
	ErrServiceAuthorityTokenExpiresAtMissing = errors.New("authz service authority token expires_at is missing")
	// ErrServiceAuthorityTokenExpired 表示取到的 service token 已经过期。
	ErrServiceAuthorityTokenExpired = errors.New("authz service authority token is expired")
)

// ServiceAuthorityConfig 描述服务间调用 service token 的主动刷新策略。
type ServiceAuthorityConfig struct {
	// RefreshBefore 表示 token 过期前多久主动刷新，例如 1m；为空使用默认值。
	RefreshBefore string `json:"refresh_before" yaml:"refresh_before"`
}

// ServiceAuthorityToken 表示 auth 服务签发的服务身份凭证及其过期时间。
type ServiceAuthorityToken struct {
	// Token 是最终写入 X-Firefly-Service-Authority 的服务 token。
	Token string
	// ExpiresAt 是 token 过期时间；目标链路要求 service token 必须可轮换，零值无效。
	ExpiresAt time.Time
}

// ServiceAuthorityFetchFunc 表示业务服务如何从 auth 服务获取 service token。
type ServiceAuthorityFetchFunc func(ctx context.Context) (*ServiceAuthorityToken, error)

// ServiceAuthorityStatus 是 service token 管理器的安全排障快照。
//
// 该结构只描述 token 是否已经装载、何时过期以及最近刷新错误，
// 绝不包含 X-Firefly-Service-Authority 的 token 原文。
type ServiceAuthorityStatus struct {
	// Started 表示后台刷新协程是否已经启动。
	Started bool `json:"started"`
	// Loaded 表示当前缓存中存在尚未过期的 service token。
	Loaded bool `json:"loaded"`
	// ExpiresAt 是当前缓存 token 的过期时间；没有可用过期时间时为空。
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// ExpiresInSeconds 表示距离 ExpiresAt 的秒数；过期 token 会显示负数。
	ExpiresInSeconds int64 `json:"expires_in_seconds"`
	// RefreshBefore 表示当前配置的过期前主动刷新窗口。
	RefreshBefore string `json:"refresh_before"`
	// LastError 是最近一次后台刷新错误；刷新成功后会清空。
	LastError string `json:"last_error,omitempty"`
}

// ServiceAuthorityProvider 表示出站调用在热路径上获取当前服务 token 的能力。
type ServiceAuthorityProvider interface {
	// ServiceAuthority 返回当前这一跳调用方服务身份 token。
	ServiceAuthority(ctx context.Context) (string, error)
}

// ServiceAuthorityStatusProvider 表示读取 service token 管理器安全状态快照的能力。
type ServiceAuthorityStatusProvider interface {
	// ServiceAuthorityStatus 返回当前 service token 管理器的安全排障快照。
	ServiceAuthorityStatus() ServiceAuthorityStatus
}

// ServiceAuthorityManager 表示具备后台获取和刷新能力的 service authority provider。
type ServiceAuthorityManager interface {
	ServiceAuthorityProvider
	ServiceAuthorityStatusProvider
	// Start 启动后台刷新协程；调用后会立即异步 fetch 一次 service token。
	Start(ctx context.Context) error
	// Stop 停止后台刷新协程；不会撤销 auth 服务已经签发的 service token。
	Stop()
}

// CachedServiceAuthorityProviderOptions 定义缓存型 service authority provider 的依赖。
type CachedServiceAuthorityProviderOptions struct {
	// Fetch 负责真正向 auth 服务签发或刷新 service token。
	Fetch ServiceAuthorityFetchFunc
	// RefreshBefore 表示 token 过期前多久主动刷新。
	RefreshBefore time.Duration
	// RetryBaseInterval 表示失败重试的基础间隔；测试可缩短，生产默认 1 分钟。
	RetryBaseInterval time.Duration
	// RetryMaxInterval 表示失败重试的最大间隔；测试可缩短，生产默认 60 分钟。
	RetryMaxInterval time.Duration
}

// CachedServiceAuthorityProvider 在进程内缓存 service token，并在过期前主动刷新。
type CachedServiceAuthorityProvider struct {
	// mu 保护 token、expiresAt 与 lastRefreshErr，避免后台刷新和出站热路径产生数据竞争。
	mu sync.RWMutex
	// fetch 保存真正获取 service token 的业务函数。
	fetch ServiceAuthorityFetchFunc
	// refreshBefore 保存提前刷新窗口，避免临界过期 token 被写入出站请求。
	refreshBefore time.Duration
	// retryBaseInterval 保存失败退避基础间隔。
	retryBaseInterval time.Duration
	// retryMaxInterval 保存失败退避最大间隔。
	retryMaxInterval time.Duration
	// token 保存最近一次成功获取的 service token。
	token string
	// expiresAt 保存 token 过期时间；目标链路要求零值无效，避免服务 token 无法轮换。
	expiresAt time.Time
	// lastRefreshErr 保存最近一次后台刷新错误，便于热路径返回可诊断的不可用错误。
	lastRefreshErr error
	// lifecycleMu 保护后台协程生命周期，避免重复 Start 或 Stop 产生多个刷新循环。
	lifecycleMu sync.Mutex
	// lifecycleCancel 保存后台刷新协程的取消函数。
	lifecycleCancel context.CancelFunc
	// lifecycleID 标记后台协程代次，避免旧协程退出时清理掉新协程状态。
	lifecycleID uint64
}

// NewServiceAuthorityProvider 根据配置和取 token 函数构造缓存型 provider。
func NewServiceAuthorityProvider(cfg *ServiceAuthorityConfig, fetch ServiceAuthorityFetchFunc) (*CachedServiceAuthorityProvider, error) {
	// 目标链路下 service authority provider 只要被构造，就必须能获取当前服务 token。
	if fetch == nil {
		return nil, ErrServiceAuthorityFetchMissing
	}
	// nil 配置表示使用默认刷新窗口，不表示禁用 service authority。
	if cfg == nil {
		cfg = &ServiceAuthorityConfig{}
	}
	// 解析刷新窗口，支持 30s、1m 等标准 duration。
	refreshBefore, err := parseServiceAuthorityRefreshBefore(cfg.RefreshBefore)
	if err != nil {
		return nil, err
	}
	// 构造可在多协程出站调用中复用的缓存 provider。
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		Fetch:         fetch,
		RefreshBefore: refreshBefore,
	})
	if err != nil {
		return nil, err
	}
	// 返回具体管理器，调用方需要在服务启动期调用 Start(ctx) 开启后台刷新。
	return provider, nil
}

// NewCachedServiceAuthorityProvider 创建缓存型 service authority provider。
func NewCachedServiceAuthorityProvider(options CachedServiceAuthorityProviderOptions) (*CachedServiceAuthorityProvider, error) {
	// 取 token 函数是唯一必需依赖。
	if options.Fetch == nil {
		return nil, ErrServiceAuthorityFetchMissing
	}
	// 未指定刷新窗口时使用默认值。
	refreshBefore := options.RefreshBefore
	if refreshBefore <= 0 {
		refreshBefore = DefaultServiceAuthorityRefreshBefore
	}
	// 未指定基础退避间隔时使用生产默认值。
	retryBaseInterval := options.RetryBaseInterval
	if retryBaseInterval <= 0 {
		retryBaseInterval = DefaultServiceAuthorityRetryBaseInterval
	}
	// 未指定最大退避间隔时使用生产默认值。
	retryMaxInterval := options.RetryMaxInterval
	if retryMaxInterval <= 0 {
		retryMaxInterval = DefaultServiceAuthorityRetryMaxInterval
	}
	// 最大退避小于基础退避时按基础退避兜底，避免配置导致退避计算为异常值。
	if retryMaxInterval < retryBaseInterval {
		retryMaxInterval = retryBaseInterval
	}
	// 返回内部状态为空的 provider，调用 Start 后由后台协程立即获取 token。
	return &CachedServiceAuthorityProvider{
		fetch:             options.Fetch,
		refreshBefore:     refreshBefore,
		retryBaseInterval: retryBaseInterval,
		retryMaxInterval:  retryMaxInterval,
	}, nil
}

// ServiceAuthority 返回当前可用的 service token。
func (p *CachedServiceAuthorityProvider) ServiceAuthority(ctx context.Context) (string, error) {
	// provider 为空时说明调用方装配错误，返回明确错误。
	if p == nil {
		return "", ErrServiceAuthorityFetchMissing
	}
	// 出站热路径只读取已经由后台协程刷新的缓存，不同步调用 auth 服务。
	if token, ok, err := p.currentToken(time.Now()); ok {
		return token, nil
	} else if err != nil {
		return "", err
	}
	// 理论上 currentToken 会在不可用时返回错误，这里保留兜底语义。
	return "", ErrServiceTokenUnavailable
}

// ServiceAuthorityStatus 返回当前 service token 管理器的安全排障快照。
func (p *CachedServiceAuthorityProvider) ServiceAuthorityStatus() ServiceAuthorityStatus {
	if p == nil {
		return ServiceAuthorityStatus{
			Loaded:    false,
			LastError: ErrServiceAuthorityFetchMissing.Error(),
		}
	}

	p.lifecycleMu.Lock()
	started := p.lifecycleCancel != nil
	p.lifecycleMu.Unlock()

	now := time.Now().UTC()
	p.mu.RLock()
	token := strings.TrimSpace(p.token)
	expiresAt := p.expiresAt
	refreshBefore := p.refreshBefore
	lastRefreshErr := p.lastRefreshErr
	p.mu.RUnlock()

	status := ServiceAuthorityStatus{
		Started:       started,
		Loaded:        token != "" && !expiresAt.IsZero() && expiresAt.After(now),
		RefreshBefore: refreshBefore.String(),
	}
	if !expiresAt.IsZero() {
		expiresAtUTC := expiresAt.UTC()
		status.ExpiresAt = &expiresAtUTC
		status.ExpiresInSeconds = int64(expiresAtUTC.Sub(now).Seconds())
	}
	if lastRefreshErr != nil {
		status.LastError = lastRefreshErr.Error()
	}
	return status
}

// Start 启动后台刷新协程，并立即异步获取一次 service token。
func (p *CachedServiceAuthorityProvider) Start(ctx context.Context) error {
	// provider 为空时说明调用方装配错误，返回明确错误。
	if p == nil {
		return ErrServiceAuthorityFetchMissing
	}
	// nil context 没有取消信号，按 Background 处理，保持启动装配宽容。
	if ctx == nil {
		ctx = context.Background()
	}
	// 生命周期锁只保护协程创建和取消函数替换。
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	// 已经启动时直接返回，避免重复创建多个刷新协程。
	if p.lifecycleCancel != nil {
		return nil
	}
	// 为后台刷新协程派生独立取消信号。
	runCtx, cancel := context.WithCancel(ctx)
	p.lifecycleID++
	lifecycleID := p.lifecycleID
	p.lifecycleCancel = cancel
	// 后台协程会立即 fetch 一次，失败后按固定退避继续无限重试。
	go p.runRefreshLoop(runCtx, lifecycleID)
	return nil
}

// Stop 停止后台刷新协程。
func (p *CachedServiceAuthorityProvider) Stop() {
	// nil provider 没有可停止的后台任务。
	if p == nil {
		return
	}
	// 取出取消函数后清空生命周期状态，允许后续重新 Start。
	p.lifecycleMu.Lock()
	cancel := p.lifecycleCancel
	p.lifecycleCancel = nil
	p.lifecycleID++
	p.lifecycleMu.Unlock()
	// 在锁外执行 cancel，避免 fetch 侧回调间接触发锁竞争。
	if cancel != nil {
		cancel()
	}
}

// runRefreshLoop 按“立即获取、成功等刷新窗口、失败按 10m 到 60m 退避”的规则运行。
func (p *CachedServiceAuthorityProvider) runRefreshLoop(ctx context.Context, lifecycleID uint64) {
	// 协程自然退出时清理生命周期状态，允许外部在父 context 取消后重新 Start。
	defer p.clearLifecycle(lifecycleID)
	// retryCount 只统计连续失败次数，成功后必须清零。
	retryCount := 0
	for {
		// 每轮一开始就尝试刷新，因此 Start 后会立即 fetch。
		refreshed := p.refreshOnce(ctx)
		// 如果外部已经取消，直接退出，不再计算下一次等待。
		if ctx.Err() != nil {
			return
		}
		// 成功后清零失败次数，并等到过期前刷新窗口。
		if refreshed {
			retryCount = 0
			if !sleepServiceAuthorityRefresh(ctx, p.nextRefreshDelay()) {
				return
			}
			continue
		}
		// 失败后递增连续失败次数，并按 10m、20m、...、60m 封顶无限重试。
		retryCount++
		if !sleepServiceAuthorityRefresh(ctx, p.retryDelay(retryCount)) {
			return
		}
	}
}

func (p *CachedServiceAuthorityProvider) clearLifecycle(lifecycleID uint64) {
	// 只允许当前代次清理自身状态，避免旧协程退出时覆盖新协程。
	p.lifecycleMu.Lock()
	defer p.lifecycleMu.Unlock()
	if p.lifecycleID == lifecycleID {
		p.lifecycleCancel = nil
	}
}

// refreshOnce 执行单次 fetch，并且只在新 token 合法时替换缓存。
func (p *CachedServiceAuthorityProvider) refreshOnce(ctx context.Context) bool {
	// 调用业务侧 fetch；fetch 自己负责用直连 auth 的方式避免 provider 递归。
	token, err := p.fetch(ctx)
	if err != nil {
		p.recordRefreshError(err)
		return false
	}
	// 统一校验 fetch 结果，避免空 token 或过期 token 污染缓存。
	if err := validateServiceAuthorityToken(token, time.Now()); err != nil {
		p.recordRefreshError(err)
		return false
	}
	// 刷新成功后写入缓存，并清掉上一轮错误。
	p.mu.Lock()
	p.token = strings.TrimSpace(token.Token)
	p.expiresAt = token.ExpiresAt.UTC()
	p.lastRefreshErr = nil
	p.mu.Unlock()
	return true
}

func (p *CachedServiceAuthorityProvider) currentToken(now time.Time) (string, bool, error) {
	// 读锁保护热路径读取，避免被后台刷新中的写入打断。
	p.mu.RLock()
	token := strings.TrimSpace(p.token)
	expiresAt := p.expiresAt
	lastRefreshErr := p.lastRefreshErr
	p.mu.RUnlock()
	// token 为空、过期时间为空或已经过期，都表示当前没有可用 service token。
	if token == "" || expiresAt.IsZero() || !expiresAt.After(now.UTC()) {
		if lastRefreshErr != nil {
			return "", false, fmt.Errorf("%w: %w", ErrServiceTokenUnavailable, lastRefreshErr)
		}
		return "", false, ErrServiceTokenUnavailable
	}
	// 只要 token 尚未过期，即使已经进入刷新窗口，也继续返回给当前出站调用。
	return token, true, nil
}

func (p *CachedServiceAuthorityProvider) recordRefreshError(err error) {
	// 空错误无需记录，避免覆盖更有价值的最近错误。
	if err == nil {
		return
	}
	// 只记录错误，不清理未过期旧 token，避免短暂网络抖动扩大为出站调用不可用。
	p.mu.Lock()
	p.lastRefreshErr = err
	p.mu.Unlock()
}

func (p *CachedServiceAuthorityProvider) nextRefreshDelay() time.Duration {
	// 读取过期时间和刷新窗口，计算下一次主动刷新时间。
	p.mu.RLock()
	expiresAt := p.expiresAt
	refreshBefore := p.refreshBefore
	p.mu.RUnlock()
	// 过期时间缺失时尽快进入下一轮刷新，避免后台协程长时间沉睡。
	if expiresAt.IsZero() {
		return time.Second
	}
	// delay 表示距离“过期前刷新窗口”的剩余时间。
	delay := time.Until(expiresAt.Add(-refreshBefore))
	// 如果 token 已经进入刷新窗口，至少等待 1 秒再重试，避免 TTL 过短时形成紧密自旋。
	if delay <= 0 {
		return time.Second
	}
	return delay
}

func (p *CachedServiceAuthorityProvider) retryDelay(retryCount int) time.Duration {
	// retryCount 小于 1 时按首次失败处理。
	if retryCount < 1 {
		retryCount = 1
	}
	// 固定业务公式：基础间隔 * 重试次数 * 10。
	delay := p.retryBaseInterval * time.Duration(retryCount) * serviceAuthorityRetryMultiplier
	// 达到最大间隔后保持最大间隔无限重试。
	if delay > p.retryMaxInterval {
		return p.retryMaxInterval
	}
	return delay
}

func sleepServiceAuthorityRefresh(ctx context.Context, delay time.Duration) bool {
	// 非正数延迟没有等待意义，调用方可以立即进入下一轮。
	if delay <= 0 {
		return true
	}
	// 使用 timer 而不是 time.Sleep，确保 Stop 或父 ctx 取消时能及时退出。
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// NewServiceAuthorityToken 根据 auth 服务返回的 token 和 expired 字符串构造统一 token。
func NewServiceAuthorityToken(token string, expired string) (*ServiceAuthorityToken, error) {
	// 先解析 expired；service token 必须有明确过期时间，便于动态轮换。
	expiresAt, err := ParseServiceAuthorityExpiresAt(expired)
	if err != nil {
		return nil, err
	}
	// 返回标准 token 结构，调用方无需关心 auth proto 的具体字段名。
	return &ServiceAuthorityToken{
		Token:     strings.TrimSpace(token),
		ExpiresAt: expiresAt,
	}, nil
}

// ParseServiceAuthorityExpiresAt 解析 auth 服务 SessionToken.expired 字段。
func ParseServiceAuthorityExpiresAt(value string) (time.Time, error) {
	// 目标链路要求 service token 必须有明确过期时间，便于调用方覆盖和动态轮换。
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, ErrServiceAuthorityTokenExpiresAtMissing
	}
	// auth 服务当前使用 RFC3339 输出过期时间。
	expiresAt, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse service authority expires_at: %w", err)
	}
	// 统一转 UTC，避免比较时受本地时区影响。
	return expiresAt.UTC(), nil
}

func validateServiceAuthorityToken(token *ServiceAuthorityToken, now time.Time) error {
	// fetch 返回 nil 说明没有拿到有效 token。
	if token == nil {
		return ErrServiceAuthorityTokenMissing
	}
	// token 为空时不能作为服务身份凭证。
	if strings.TrimSpace(token.Token) == "" {
		return ErrServiceAuthorityTokenMissing
	}
	// 非零过期时间必须晚于当前时间。
	if token.ExpiresAt.IsZero() {
		return ErrServiceAuthorityTokenExpiresAtMissing
	}
	// 过期时间必须晚于当前时间。
	if !token.ExpiresAt.After(now.UTC()) {
		return ErrServiceAuthorityTokenExpired
	}
	// 校验通过后允许写入缓存。
	return nil
}

func parseServiceAuthorityRefreshBefore(value string) (time.Duration, error) {
	// 未配置时使用默认刷新窗口。
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultServiceAuthorityRefreshBefore, nil
	}
	// 使用标准 duration 解析，保持与已有 clock_skew 配置一致。
	refreshBefore, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse service authority refresh_before: %w", err)
	}
	// 非正数没有刷新意义，目标链路直接拒绝错误配置。
	if refreshBefore <= 0 {
		return 0, fmt.Errorf("parse service authority refresh_before: value must be positive")
	}
	// 返回调用方配置的刷新窗口。
	return refreshBefore, nil
}
