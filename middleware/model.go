package middleware

import (
	"net/http"
)

type HttpAdapter func(handler http.Handler) http.Handler

type HttpStatusResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *HttpStatusResponseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.status = code
}
