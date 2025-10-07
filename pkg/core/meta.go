package micro

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/grpc/metadata"
)

// Meta 服务元信息
type Meta struct {
	Env     string `json:"env"`
	AppId   string `json:"app_id"`
	Version string `json:"version"`
}

// UserContextMeta 用户上下文元信息
type UserContextMeta struct {
	Session  string `json:"session"`
	ClientIp string `json:"client_ip"`

	Roles []string `json:"roles"`

	UserId uuid.UUID `json:"user_id"`

	OrgId    uuid.UUID `json:"org_id"`
	AppId    uuid.UUID `json:"app_id"`
	TenantId uuid.UUID `json:"tenant_id"`
}

type ClientContextMeta struct {
	Lang       string `json:"lang"`
	ClientIp   string `json:"client_ip"`
	AppVersion string `json:"app_version"`
}

// ParseMetaKey 解析元信息key
func ParseMetaKey(md metadata.MD, key string) (string, error) {
	val := md.Get(key)

	if len(val) == 0 {
		return "", errors.New(fmt.Sprintf("%s parse error", key))
	}

	return val[0], nil
}

// ParseUserContextMeta 解析用户上下文元信息
func ParseUserContextMeta(md metadata.MD) (raw *UserContextMeta, err error) {
	var ust, ast, tst string

	raw = &UserContextMeta{}

	raw.Session, err = ParseMetaKey(md, "session")
	if err != nil {
		return nil, err
	}
	raw.ClientIp, err = ParseMetaKey(md, "client-ip")
	if err != nil {
		return nil, err
	}

	raw.Roles = md.Get("roles")

	ust, err = ParseMetaKey(md, "user-id")
	if err != nil {
		return nil, err
	}
	ast, err = ParseMetaKey(md, "app-id")
	if err != nil {
		return nil, err
	}
	tst, err = ParseMetaKey(md, "tenant-id")
	if err != nil {
		return nil, err
	}

	raw.UserId, err = uuid.Parse(ust)
	if err != nil {
		return nil, errors.New("parse user-id uuid error")
	}
	raw.AppId, err = uuid.Parse(ast)
	if err != nil {
		return nil, errors.New("parse app-id uuid error")
	}
	raw.TenantId, err = uuid.Parse(tst)
	if err != nil {
		return nil, errors.New("parse tenant-id uuid error")
	}

	return raw, nil
}

// ParseClientContextMeta 解析客户端上下文元信息
func ParseClientContextMeta(md metadata.MD) (raw *ClientContextMeta, err error) {
	raw = &ClientContextMeta{}
	raw.Lang, err = ParseMetaKey(md, "lang")
	if err != nil {
		return nil, err
	}
	raw.ClientIp, err = ParseMetaKey(md, "client-ip")
	if err != nil {
		return nil, err
	}
	raw.AppVersion, err = ParseMetaKey(md, "app-version")
	if err != nil {
		return nil, err
	}

	return raw, nil
}
