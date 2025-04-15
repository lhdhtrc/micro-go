package micro

type GatewayConf struct {
	// 网卡
	Network string `json:"network"`
	// 外网地址
	OuterNetAddr string `json:"outer_net_addr"`
	// 内网地址
	InternalNetAddr string `json:"internal_net_addr"`
}
