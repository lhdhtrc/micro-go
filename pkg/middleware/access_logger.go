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

func GrpcAccessLogger(handle func(b []byte, msg string)) grpc.UnaryServerInterceptor {
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

			loggerMap["method"] = 5
			loggerMap["path"] = info.FullMethod

			request, _ := json.Marshal(req)
			loggerMap["request"] = string(request)

			response, _ := json.Marshal(resp)
			loggerMap["response"] = string(response)

			loggerMap["duration"] = elapsed.Nanoseconds()
			loggerMap["status"] = 200

			ip := md.Get("client-ip")
			if len(ip) != 0 {
				loggerMap["ip"] = ip[0]
			}

			systemType := md.Get("system-type")
			if len(systemType) != 0 {
				loggerMap["system_type"] = systemType[0]
			} else {
				loggerMap["system_type"] = 0
			}

			systemName := md.Get("system-name")
			if len(systemName) != 0 {
				loggerMap["system_name"] = systemName[0]
			}

			systemVersion := md.Get("system-version")
			if len(systemVersion) != 0 {
				loggerMap["system_version"] = systemVersion[0]
			}

			clientType := md.Get("client-type")
			if len(clientType) != 0 {
				loggerMap["client_type"] = clientType[0]
			} else {
				loggerMap["client_type"] = 0
			}

			clientName := md.Get("client-name")
			if len(clientName) != 0 {
				loggerMap["client_name"] = clientName[0]
			}

			clientVersion := md.Get("client-version")
			if len(clientVersion) != 0 {
				loggerMap["client_version"] = clientVersion[0]
			}

			appId := md.Get("app-id")
			if len(appId) != 0 {
				loggerMap["app_id"] = appId[0]
			}

			appVersion := md.Get("app-version")
			if len(appVersion) != 0 {
				loggerMap["app_version"] = appVersion[0]
			}

			deviceFormFactor := md.Get("device-form-factor")
			if len(deviceFormFactor) != 0 {
				loggerMap["device_form_factor"] = deviceFormFactor[0]
			} else {
				loggerMap["device_form_factor"] = 0
			}

			traceId := md.Get("trace-id")
			if len(traceId) != 0 {
				loggerMap["trace_id"] = traceId[0]
			} else {
				loggerMap["trace_id"] = uuid.New().String()
			}

			b, _ := json.Marshal(loggerMap)
			handle(b, fmt.Sprintf("[%s] [GRPC]:[%s] [%s]-[%d]\n", time.Now().Format(time.DateTime), info.FullMethod, elapsed.String(), status))
		}

		return resp, err
	}
}
