package server

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"universal-curriculum/internal/db"
	"universal-curriculum/internal/models"
)

const (
	apiDefaultLimit = 25
	apiMaximumLimit = 100
	apiRequestLimit = 1 << 20
)

var apiRequestMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodDelete,
	http.MethodPatch,
	http.MethodOptions,
	http.MethodConnect,
	http.MethodTrace,
}

// openAPIContract is a generated delivery copy of docs/openapi.yaml.
//
//go:embed openapi.yaml
var openAPIContract []byte

type apiErrorBody struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type apiPage struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Count   int  `json:"count"`
	HasMore bool `json:"has_more"`
}

type apiUserContextKey struct{}

func (server *Server) apiInfo(writer http.ResponseWriter, _ *http.Request) {
	writeAPIJSON(writer, http.StatusOK, map[string]string{
		"name":          "Universal Curriculum API",
		"release":       "0.2.0",
		"status":        "experimental",
		"documentation": "/api/openapi.yaml",
	})
}

func (server *Server) apiOpenAPI(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	writer.Header().Set("Cache-Control", "public, max-age=3600")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(openAPIContract)
}

func (server *Server) apiNotFound(writer http.ResponseWriter, _ *http.Request) {
	writeAPIError(writer, http.StatusNotFound, "not_found", "The API resource was not found.", nil)
}

func registerAPIRoute(mux *http.ServeMux, path string, handlers map[string]http.Handler) {
	allowed := make([]string, 0, len(handlers)+1)
	for _, method := range apiRequestMethods {
		handler := handlers[method]
		if handler != nil {
			mux.Handle(method+" "+path, handler)
			allowed = append(allowed, method)
			if method == http.MethodGet {
				allowed = append(allowed, http.MethodHead)
			}
			continue
		}
		if method == http.MethodHead && handlers[http.MethodGet] != nil {
			continue
		}
	}

	allow := strings.Join(allowed, ", ")
	methodNotAllowed := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Allow", allow)
		writeAPIError(writer, http.StatusMethodNotAllowed, "method_not_allowed", "The method is not supported.", nil)
	})
	for _, method := range apiRequestMethods {
		if handlers[method] != nil || method == http.MethodHead && handlers[http.MethodGet] != nil {
			continue
		}
		mux.Handle(method+" "+path, methodNotAllowed)
	}
}

func (server *Server) requireAPIToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		parts := strings.Fields(request.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="api"`)
			writeAPIError(writer, http.StatusUnauthorized, "unauthorized", "A valid bearer token is required.", nil)
			return
		}
		user, err := db.AuthenticateAPIToken(server.Database, parts[1])
		if err != nil {
			log.Printf("authenticate API token: %v", err)
			writeAPIInternalError(writer)
			return
		}
		if user == nil {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="api", error="invalid_token"`)
			writeAPIError(writer, http.StatusUnauthorized, "unauthorized", "A valid bearer token is required.", nil)
			return
		}
		writer.Header().Set("Cache-Control", "private, no-store")
		ctx := withAPIUser(request, user)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func (server *Server) requireAPIAdmin(next http.Handler) http.Handler {
	return server.requireAPIToken(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		user := apiUser(request)
		if user == nil || !user.IsAdmin {
			writeAPIError(writer, http.StatusForbidden, "forbidden", "Administrator access is required.", nil)
			return
		}
		next.ServeHTTP(writer, request)
	}))
}

func apiUser(request *http.Request) *models.User {
	user, _ := request.Context().Value(apiUserContextKey{}).(*models.User)
	return user
}

func withAPIUser(request *http.Request, user *models.User) context.Context {
	return context.WithValue(request.Context(), apiUserContextKey{}, user)
}

func writeAPIJSON(writer http.ResponseWriter, status int, value any) {
	if status == http.StatusNoContent || value == nil {
		writer.WriteHeader(status)
		return
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		log.Printf("encode API response: %v", err)
	}
}

func writeAPIError(writer http.ResponseWriter, status int, code, message string, fields map[string]string) {
	writeAPIJSON(writer, status, apiErrorBody{Error: apiError{
		Code: code, Message: message, Fields: fields,
	}})
}

func writeAPIInternalError(writer http.ResponseWriter) {
	writeAPIError(writer, http.StatusInternalServerError, "internal_error", "The request could not be completed.", nil)
}

func decodeAPIJSON(writer http.ResponseWriter, request *http.Request, target any) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return errors.New("Content-Type must be application/json")
	}
	request.Body = http.MaxBytesReader(writer, request.Body, apiRequestLimit)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON value")
	}
	return nil
}

func validateAPIQuery(request *http.Request, allowed ...string) error {
	allowedSet := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = true
	}
	for key, values := range request.URL.Query() {
		if !allowedSet[key] {
			return fmt.Errorf("unsupported query parameter %q", key)
		}
		if len(values) != 1 {
			return fmt.Errorf("query parameter %q must occur once", key)
		}
	}
	return nil
}

func apiPagination(request *http.Request) (int, int, error) {
	limit := apiDefaultLimit
	offset := 0
	var err error
	if raw := request.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > apiMaximumLimit {
			return 0, 0, errors.New("limit must be an integer between 1 and 100")
		}
	}
	if raw := request.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			return 0, 0, errors.New("offset must be a non-negative integer")
		}
	}
	return limit, offset, nil
}

func apiPathID(request *http.Request, name string) (int64, error) {
	return parsePositiveID(request.PathValue(name))
}

func apiPageFor(limit, offset, count, total int) apiPage {
	return apiPage{Limit: limit, Offset: offset, Count: count, HasMore: offset+count < total}
}
