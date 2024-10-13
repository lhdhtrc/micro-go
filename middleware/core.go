package middleware

import "net/http"

func HttpUse(h http.Handler, middlewares ...HttpAdapter) http.Handler {
	for _, adapter := range middlewares {
		h = adapter(h)
	}
	return h
}
