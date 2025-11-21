package middleware

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// PropagateIncomingMetadata 将入站元数据传播到出站上下文中
func PropagateIncomingMetadata(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	oc := metadata.NewOutgoingContext(ctx, md.Copy())
	return handler(oc, req)
}
