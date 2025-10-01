package micro

import (
	"errors"
	"fmt"
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

	UserId   string `json:"user_id"`
	DeptId   string `json:"dept_id"`
	OrgId    string `json:"org_id"`
	AppId    string `json:"app_id"`
	TenantId string `json:"tenant_id"`
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
	raw = &UserContextMeta{}
	raw.Session, err = ParseMetaKey(md, "session")
	if err != nil {
		return nil, err
	}
	raw.ClientIp, err = ParseMetaKey(md, "client-ip")
	if err != nil {
		return nil, err
	}

	raw.UserId, err = ParseMetaKey(md, "user-id")
	if err != nil {
		return nil, err
	}
	raw.DeptId, err = ParseMetaKey(md, "dept-id")
	if err != nil {
		return nil, err
	}
	raw.OrgId, err = ParseMetaKey(md, "org-id")
	if err != nil {
		return nil, err
	}
	raw.AppId, err = ParseMetaKey(md, "app-id")
	if err != nil {
		return nil, err
	}
	raw.TenantId, err = ParseMetaKey(md, "tenant-id")
	if err != nil {
		return nil, err
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
