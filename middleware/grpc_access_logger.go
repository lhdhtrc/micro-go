package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"time"
)

func GrpcAccessLogger(handle func(b []byte), console bool) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		md, _ := metadata.FromIncomingContext(ctx)

		// 调用下一个拦截器或服务方法
		resp, err := handler(ctx, req)

		status := 200
		elapsed := time.Since(start)

		if err != nil {
			status = 400
		}

		if handle != nil {
			loggerMap := make(map[string]interface{})

			traceId := md.Get("TraceId")
			if len(traceId) != 0 {
				loggerMap["trace_id"] = traceId[0]
			} else {
				loggerMap["trace_id"] = uuid.New().String()
			}

			ip := md.Get("x-real-ip")
			if len(ip) != 0 {
				loggerMap["ip"] = ip[0]
			}

			clientName := md.Get("ClientName")
			if len(clientName) != 0 {
				loggerMap["client_name"] = clientName[0]
			}

			clientType := md.Get("ClientType")
			if len(clientName) != 0 {
				loggerMap["client_type"] = clientType[0]
			}

			clientSystem := md.Get("ClientSystem")
			if len(clientSystem) != 0 {
				loggerMap["client_system"] = clientSystem[0]
			}

			loggerMap["status"] = 200
			loggerMap["timer"] = elapsed.String()

			loggerMap["method"] = "grpc"
			loggerMap["path"] = info.FullMethod

			request, _ := json.Marshal(req)
			loggerMap["request"] = string(request)

			response, _ := json.Marshal(resp)
			loggerMap["response"] = string(response)

			b, _ := json.Marshal(loggerMap)
			handle(b)
		}

		if console {
			fmt.Printf("[%s] [GRPC]:[%s] [%s]-[%d]\n", time.Now().Format(time.DateTime), info.FullMethod, elapsed.String(), status)
		}

		return resp, err
	}
}
