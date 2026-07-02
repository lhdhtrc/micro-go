package telemetry

// Config 定义了 Telemetry 的配置结构。
// 它可以映射到配置文件（如 JSON/YAML）中的 telemetry 字段。
type Config struct {
	// OTLPEndpoint 是 OpenTelemetry Collector 的地址 (host:port)。
	OTLPEndpoint string `json:"otlp_endpoint"`
	// Insecure 决定是否使用非安全连接 (HTTP/gRPC without TLS)。
	Insecure bool `json:"insecure"`

	// Traces 开关，控制是否启用链路追踪。
	Traces bool `json:"traces"`
	// Logs 开关，控制是否启用日志收集。
	Logs bool `json:"logs"`
	// Metrics 开关，控制是否启用指标监控。
	Metrics bool `json:"metrics"`
}

type Resource struct {
	// ServiceId 服务id
	ServiceId string `json:"service_id"`
	// Environment 服务运行环境，例如 dev、test、prod。
	Environment string `json:"environment"`
	// ServiceName 服务名称
	ServiceName string `json:"service_name"`
	// ServiceVersion 服务版本
	ServiceVersion string `json:"service_version"`
	// ServiceNamespace 服务命名空间
	ServiceNamespace string `json:"service_namespace"`
	// ServiceInstanceId 服务实例id
	ServiceInstanceId string `json:"service_instance_id"`
}
