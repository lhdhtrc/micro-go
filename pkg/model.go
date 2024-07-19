package micro

import (
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type CoreEntity struct {
	server *grpc.Server
	logger *zap.Logger
}
