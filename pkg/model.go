package micro

import (
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type CoreEntity struct {
	logger *zap.Logger

	Server *grpc.Server
}
