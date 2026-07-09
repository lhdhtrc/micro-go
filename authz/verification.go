package authz

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fireflycore/go-micro/constant"
	"github.com/fireflycore/go-micro/service"
)

const (
	// DefaultKid 是当前 authz JWS 默认使用的固定 kid。
	DefaultKid = constant.AuthzSignDefaultKid
	// DefaultIssuer 是当前 authz JWS 默认签发方。
	DefaultIssuer = constant.AuthzSignDefaultIssuer
	// DefaultClockSkew 是服务侧验签允许的默认时钟偏差。
	DefaultClockSkew = 5 * time.Second
)

// VerificationConfig 描述业务服务如何本地验签 authz 注入的 x-firefly-authz-sign。
type VerificationConfig struct {
	// Kid 是当前使用的公钥 ID；为空时使用 DefaultKid。
	Kid string `json:"kid" yaml:"kid"`
	// PublicKeyPath 是 Ed25519 公钥 PEM 文件路径。
	PublicKeyPath string `json:"public_key_path" yaml:"public_key_path"`
	// Issuer 是期望的 JWS iss；为空时使用 DefaultIssuer。
	Issuer string `json:"issuer" yaml:"issuer"`
	// ClockSkew 是允许的时钟偏差，例如 5s。
	ClockSkew string `json:"clock_skew" yaml:"clock_skew"`
	// SkipMethods 是不执行 authz 上下文验签的 gRPC FullMethod 列表。
	SkipMethods []string `json:"skip_methods" yaml:"skip_methods"`
}

// VerificationOptions 是业务服务接入 go-micro gRPC middleware 时需要的验签配置。
type VerificationOptions struct {
	// AuthzVerification 是传给 ServiceContextUnaryInterceptor 的验签规则。
	AuthzVerification *service.AuthzSignVerificationOptions
	// AuthzSkipMethods 是传给 ServiceContextUnaryInterceptor 的跳过验签方法列表。
	AuthzSkipMethods []string
}

// NewVerificationOptions 根据业务服务配置构造 go-micro 服务侧验签选项。
func NewVerificationOptions(cfg *VerificationConfig) (*VerificationOptions, error) {
	// 未配置时表示调用方没有装配服务侧验签，返回空选项供启动层显式处理。
	if cfg == nil {
		return &VerificationOptions{}, nil
	}

	// kid 为空时使用当前固定默认值。
	kid := strings.TrimSpace(cfg.Kid)
	if kid == "" {
		kid = DefaultKid
	}
	// issuer 为空时使用当前 authz 固定签发方。
	issuer := strings.TrimSpace(cfg.Issuer)
	if issuer == "" {
		issuer = DefaultIssuer
	}
	// 启用验签后必须显式配置公钥路径。
	publicKeyPath := strings.TrimSpace(cfg.PublicKeyPath)
	if publicKeyPath == "" {
		return nil, fmt.Errorf("authz verification public_key_path is required")
	}

	// 从 PEM 文件加载 Ed25519 公钥。
	publicKey, err := LoadEd25519PublicKey(publicKeyPath)
	if err != nil {
		return nil, err
	}
	// 解析允许的时钟偏差。
	clockSkew, err := parseClockSkew(cfg.ClockSkew)
	if err != nil {
		return nil, err
	}

	// PublicKeys 使用 kid 索引，虽然当前只有 default，但保持与 service 验签模型一致。
	verification := &service.AuthzSignVerificationOptions{
		PublicKeys: map[string]ed25519.PublicKey{
			kid: publicKey,
		},
		Issuer:    issuer,
		ClockSkew: clockSkew,
	}

	// 返回 middleware 可直接使用的 options。
	return &VerificationOptions{
		AuthzVerification: verification,
		AuthzSkipMethods:  cloneStrings(cfg.SkipMethods),
	}, nil
}

// MustNewVerificationOptions 是 NewVerificationOptions 的启动期便捷封装。
func MustNewVerificationOptions(cfg *VerificationConfig) *VerificationOptions {
	// 启动期配置错误应快速失败。
	options, err := NewVerificationOptions(cfg)
	if err != nil {
		panic(err)
	}
	// 返回已解析的验签选项。
	return options
}

// LoadEd25519PublicKey 从 PEM 文件加载 Ed25519 公钥。
func LoadEd25519PublicKey(path string) (ed25519.PublicKey, error) {
	// 读取 openssl pkey -pubout 生成的 PEM 公钥文件。
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read authz public key %s: %w", path, err)
	}
	// 解码 PEM block。
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("decode authz public key %s: missing PEM block", path)
	}
	// PKIX 是 openssl pkey -pubout 的标准公钥格式。
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse authz public key %s: %w", path, err)
	}
	// 当前 authz JWS 使用 EdDSA/Ed25519，其他算法直接拒绝。
	publicKey, ok := parsed.(ed25519.PublicKey)
	if !ok || len(publicKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("parse authz public key %s: expected Ed25519 public key", path)
	}
	// 返回可直接用于 JWS 验签的 Ed25519 公钥。
	return publicKey, nil
}

func parseClockSkew(value string) (time.Duration, error) {
	// 未配置时使用默认时钟偏差。
	value = strings.TrimSpace(value)
	if value == "" {
		return DefaultClockSkew, nil
	}
	// 使用 time.ParseDuration 支持 5s、1m 等标准 duration 字符串。
	clockSkew, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse authz verification clock_skew: %w", err)
	}
	// 负数时钟偏差没有业务意义，直接拒绝错误配置。
	if clockSkew < 0 {
		return 0, fmt.Errorf("parse authz verification clock_skew: value must be non-negative")
	}
	// 返回调用方配置的时钟偏差。
	return clockSkew, nil
}

func cloneStrings(values []string) []string {
	// 空切片直接返回 nil，减少后续配置判断分支。
	if len(values) == 0 {
		return nil
	}
	// 复制切片，避免调用方后续修改配置影响 middleware。
	cloned := make([]string, 0, len(values))
	for _, value := range values {
		// 跳过空 method，避免误配置无意义跳过规则。
		if value = strings.TrimSpace(value); value != "" {
			cloned = append(cloned, value)
		}
	}
	// 返回独立切片。
	return cloned
}
