package werror

import (
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
)

var reasonPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,62}$`)

// Definition 定义一个可复用、可枚举的稳定业务错误。
type Definition struct {
	Code    codes.Code `json:"code"`
	Domain  string     `json:"domain"`
	Reason  string     `json:"reason"`
	Message string     `json:"message"`
}

// Key 返回适合作为错误目录索引的 domain/reason 组合键。
func (definition Definition) Key() string {
	return definition.Domain + "/" + definition.Reason
}

// Validate 校验错误定义是否满足 ErrorInfo 和默认降级消息约束。
func (definition Definition) Validate() error {
	if strings.TrimSpace(definition.Domain) == "" || strings.ContainsAny(definition.Domain, " \t\r\n") {
		return fmt.Errorf("werror: domain is required and must not contain whitespace")
	}
	if !reasonPattern.MatchString(definition.Reason) || strings.HasSuffix(definition.Reason, "_") {
		return fmt.Errorf("werror: reason %q must be UPPER_SNAKE_CASE and at most 63 characters", definition.Reason)
	}
	if definition.Code == codes.OK {
		return fmt.Errorf("werror: code must not be OK")
	}
	if strings.TrimSpace(definition.Message) == "" {
		return fmt.Errorf("werror: fallback message is required")
	}
	return nil
}

// New 按定义创建结构化错误；domain 和 reason 始终以定义为准。
func (definition Definition) New(options ...Option) *Error {
	resolved := make([]Option, 0, len(options)+2)
	resolved = append(resolved, options...)
	resolved = append(resolved, WithDomain(definition.Domain), WithReason(definition.Reason))
	return New(definition.Code, definition.Message, resolved...)
}

// Wrap 按定义包装底层错误；domain 和 reason 始终以定义为准。
func (definition Definition) Wrap(cause error, options ...Option) *Error {
	resolved := make([]Option, 0, len(options)+1)
	resolved = append(resolved, options...)
	if cause != nil {
		resolved = append(resolved, WithCause(cause))
	}
	return definition.New(resolved...)
}

// Catalog 保存一个服务内不可重复的业务错误目录。
type Catalog struct {
	definitions []Definition
	index       map[string]Definition
}

// NewCatalog 创建错误目录，并校验定义完整性以及 domain/reason 唯一性。
func NewCatalog(definitions ...Definition) (*Catalog, error) {
	catalog := &Catalog{
		definitions: make([]Definition, 0, len(definitions)),
		index:       make(map[string]Definition, len(definitions)),
	}
	for _, definition := range definitions {
		if err := definition.Validate(); err != nil {
			return nil, err
		}
		key := catalogIndexKey(definition.Domain, definition.Reason)
		if _, exists := catalog.index[key]; exists {
			return nil, fmt.Errorf("werror: duplicate definition %s", definition.Key())
		}
		catalog.definitions = append(catalog.definitions, definition)
		catalog.index[key] = definition
	}
	return catalog, nil
}

// MustCatalog 创建错误目录，定义不合法时 panic，适合服务包级初始化。
func MustCatalog(definitions ...Definition) *Catalog {
	catalog, err := NewCatalog(definitions...)
	if err != nil {
		panic(err)
	}
	return catalog
}

// Definitions 返回错误目录定义的副本。
func (catalog *Catalog) Definitions() []Definition {
	if catalog == nil || len(catalog.definitions) == 0 {
		return nil
	}
	definitions := make([]Definition, len(catalog.definitions))
	copy(definitions, catalog.definitions)
	return definitions
}

// Find 按 domain/reason 查询错误定义。
func (catalog *Catalog) Find(domain, reason string) (Definition, bool) {
	if catalog == nil {
		return Definition{}, false
	}
	definition, ok := catalog.index[catalogIndexKey(domain, reason)]
	return definition, ok
}

func catalogIndexKey(domain, reason string) string {
	return domain + "\x00" + reason
}
