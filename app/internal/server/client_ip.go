package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type trustRenderProxyHeadersContextKey struct{}

func ClientIP(request *http.Request) (netip.Addr, error) {
	if trustProxy, _ := request.Context().Value(trustRenderProxyHeadersContextKey{}).(bool); trustProxy {
		if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-For")); forwarded != "" {
			candidate := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			address, err := netip.ParseAddr(candidate)
			if err != nil {
				return netip.Addr{}, fmt.Errorf("invalid forwarded client IP: %w", err)
			}
			return address.Unmap(), nil
		}
	}

	host := strings.TrimSpace(request.RemoteAddr)
	if splitHost, _, err := net.SplitHostPort(host); err == nil {
		host = splitHost
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid remote client IP: %w", err)
	}
	return address.Unmap(), nil
}

func configureClientIP(server *Server, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctx := context.WithValue(
			request.Context(),
			trustRenderProxyHeadersContextKey{},
			server.Config.TrustRenderProxyHeaders,
		)
		handler.ServeHTTP(writer, request.WithContext(ctx))
	})
}
