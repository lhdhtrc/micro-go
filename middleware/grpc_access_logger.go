package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"time"
)

var accessLoggerMetadataKeys = []string{"ClientName", "ClientType", "ClientSystem", "Ip", "Address", "AccountId", "AppId"}

var accessLoggerClientTypeMap = map[string]int{
	"浏览器":   1,
	"桌面软件":  2,
	"APP应用": 3,
	"服务调用":  4,
}
var accessLoggerClientSystemMap = map[string]int{
	"windows": 1,
	"macos":   2,
	"linux":   3,
	"android": 4,
	"ios":     5,
}

func GrpcAccessLogger(handle func(b []byte)) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		md, _ := metadata.FromIncomingContext(ctx)

		// 调用下一个拦截器或服务方法
		resp, err := handler(ctx, req)

		elapsed := time.Since(start)
		duration := float64(elapsed.Nanoseconds()) / 1e6

		loggerMap := make(map[string]interface{})
		for _, key := range accessLoggerMetadataKeys {
			item := md.Get(key)
			if len(item) != 0 {
				if key == "ClientType" {
					if val, ok := accessLoggerClientTypeMap[item[0]]; ok {
						loggerMap[key] = val
					} else {
						loggerMap[key] = 0
					}
				} else if key == "ClientSystem" {
					if val, ok := accessLoggerClientSystemMap[item[0]]; ok {
						loggerMap[key] = val
					} else {
						loggerMap[key] = 0
					}
				} else {
					loggerMap[key] = item[0]
				}
			} else {
				if key == "ClientType" || key == "ClientSystem" {
					loggerMap[key] = 0
				} else {
					loggerMap[key] = ""
				}
			}
		}

		loggerMap["Method"] = "grpc"
		loggerMap["Path"] = info.FullMethod
		loggerMap["Request"] = req
		loggerMap["Response"] = resp
		loggerMap["Status"] = 200
		loggerMap["Timer"] = fmt.Sprintf("%.3fms", duration)

		b, _ := json.Marshal(loggerMap)
		if handle != nil {
			handle(b)
		}

		return resp, err
	}
}
