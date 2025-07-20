package middleware

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// GrpcContextConverter 将入站上下文转换为出战上下文
func GrpcContextConverter(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	oc := metadata.NewOutgoingContext(ctx, md.Copy())
	return handler(oc, req)
}
