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
	Namespace string `json:"namespace"`
	Endpoint  string `json:"endpoint"`
	MaxRetry  uint   `json:"max_retry"`
	TTL       int64  `json:"ttl"`
	DNS       string `json:"dns"`
	Run       string `json:"run"`
}
