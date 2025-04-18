package micro

// Meta 服务元信息
type Meta struct {
	Env     string `json:"env"`
	AppId   string `json:"app_id"`
	Version string `json:"version"`
}
