package micro

import "google.golang.org/grpc"

func (core *CoreEntity) Server() *grpc.Server {
	return core.server
}
