package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func HttpAccessLogger(handle func(b []byte), console bool) HttpAdapter {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			start := time.Now()
			sw := &HttpStatusResponseWriter{writer, http.StatusOK}
			//store.Use.Logger.Info(fmt.Sprintf("%s %s start ------>", request.Method, request.URL.Path))

			next.ServeHTTP(sw, request)

			status := sw.status
			elapsed := time.Since(start)

			if console {
				// todo 控制台输出
			}

			if handle != nil {
				loggerMap := make(map[string]interface{})

				fmt.Println(request.URL.RawQuery)
				fmt.Println(request.MultipartForm.Value)
				fmt.Println(request.Header)
				fmt.Println(request.Body)

				loggerMap["Method"] = request.Method
				loggerMap["Path"] = request.URL.Path
				loggerMap["Request"] = request.URL.RawQuery
				loggerMap["Response"] = ""
				loggerMap["Status"] = status
				loggerMap["Timer"] = elapsed.String()

				b, _ := json.Marshal(loggerMap)
				handle(b)
			}

			//store.Use.Logger.Info(fmt.Sprintf("%s %s end <------ %s %d", request.Method, request.URL.Path, duration.String(), code))
		})
	}
}
