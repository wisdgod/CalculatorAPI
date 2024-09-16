package utils

import (
	"net/http"
)

type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode int
}

func (rw *ResponseWriterWrapper) WriteHeader(code int) {
	rw.StatusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
