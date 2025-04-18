package micro

// ServiceConf 服务注册/服务发现配置
type ServiceConf struct {
	// 命名控件
	Namespace string `json:"namespace" bson:"namespace" yaml:"namespace" mapstructure:"namespace"`
	// 网卡
	Network *Network `json:"network"`
	// 内核
	Kernel *Kernel `json:"kernel"`

	// 最大重试次数
	MaxRetry uint32 `json:"max_retry" bson:"max_retry" yaml:"max_retry" mapstructure:"max_retry"`
	// 心跳间隔
	TTL uint32 `json:"ttl" bson:"ttl" yaml:"ttl" mapstructure:"ttl"`
}
