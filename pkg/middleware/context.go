package middleware

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// GrpcContextConversionInterceptor 创建出站上下文
func GrpcContextConversionInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	oc := metadata.NewOutgoingContext(ctx, md.Copy())
	return handler(oc, req)
}
