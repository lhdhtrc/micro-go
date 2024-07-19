package micro

import (
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type CoreEntity struct {
	server *grpc.Server
	logger *zap.Logger
}

type ConfigEntity struct {
	Namespace string `json:"namespace" yaml:"namespace" mapstructure:"namespace"`
	Endpoint  string `json:"endpoint" yaml:"endpoint" mapstructure:"endpoint"`
	DNS       string `json:"dns" yaml:"dns" mapstructure:"dns"`
	Run       string `json:"run" yaml:"run" mapstructure:"run"`
}
