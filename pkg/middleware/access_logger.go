package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	micro "github.com/lhdhtrc/micro-go/pkg/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"strconv"
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

			loggerMap["duration"] = elapsed.String()
			loggerMap["status"] = 200

			loggerMap["ip"], err = micro.ParseMetaKey(md, "client-ip")

			loggerMap["system_name"], _ = micro.ParseMetaKey(md, "system-name")
			loggerMap["client_name"], _ = micro.ParseMetaKey(md, "client-name")

			var systemType, clientType, deviceFormFactor string
			systemType, err = micro.ParseMetaKey(md, "system-type")
			if err == nil {
				loggerMap["system_type"] = 0
			} else {
				loggerMap["system_type"], _ = strconv.ParseInt(systemType, 10, 32)
			}
			clientType, err = micro.ParseMetaKey(md, "client-type")
			if err == nil {
				loggerMap["client_type"] = 0
			} else {
				loggerMap["client_type"], _ = strconv.ParseInt(clientType, 10, 32)
			}
			deviceFormFactor, err = micro.ParseMetaKey(md, "device-form-factor")
			if err == nil {
				loggerMap["device_form_factor"] = deviceFormFactor
			} else {
				loggerMap["device_form_factor"] = 0
			}

			loggerMap["system_version"], _ = micro.ParseMetaKey(md, "system-version")
			loggerMap["client_version"], _ = micro.ParseMetaKey(md, "client-version")
			loggerMap["app_version"], _ = micro.ParseMetaKey(md, "app-version")

			loggerMap["trace_id"], err = micro.ParseMetaKey(md, "trace-id")
			if err != nil {
				loggerMap["trace_id"] = uuid.New().String()
			}
			loggerMap["app_id"], _ = micro.ParseMetaKey(md, "app-id")

			b, _ := json.Marshal(loggerMap)
			handle(b, fmt.Sprintf("[%s] [GRPC]:[%s] [%s]-[%d]\n", time.Now().Format(time.DateTime), info.FullMethod, elapsed.String(), status))
		}

		return resp, err
	}
}
