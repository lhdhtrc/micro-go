package micro

type Server struct {
	Port uint32 `json:"port" bson:"port" yaml:"port" mapstructure:"port"`
	Addr string `json:"addr" bson:"addr" yaml:"addr" mapstructure:"addr"`
}

type Gateway struct {
	OuterNetAccessAddr string `json:"outer_net_access_addr" bson:"outer_net_access_addr" yaml:"outer_net_access_addr" mapstructure:"outer_net_access_addr"`
	InterNetAccessAddr string `json:"inter_net_access_addr" bson:"inter_net_access_addr" yaml:"inter_net_access_addr" mapstructure:"inter_net_access_addr"`
}

type ServiceConfig struct {
	Mode      bool   `json:"mode" bson:"mode" yaml:"mode" mapstructure:"mode"`
	AppId     string `json:"app_id" bson:"app_id" yaml:"app_id" mapstructure:"app_id"`
	Namespace string `json:"namespace" bson:"namespace" yaml:"namespace" mapstructure:"namespace"`
	MaxRetry  uint32 `json:"max_retry" bson:"max_retry" yaml:"max_retry" mapstructure:"max_retry"`
	TTL       uint32 `json:"ttl" bson:"ttl" yaml:"ttl" mapstructure:"ttl"`
}
