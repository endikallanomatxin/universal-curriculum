package server

import "net/http"

func sensitiveAuthResponse(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		handler.ServeHTTP(writer, request)
	})
}
