package server

import (
	"fmt"
	"net/http"
	"strconv"

	"universal-curriculum/internal/services"
)

func parsePositiveID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid positive ID")
	}
	return id, nil
}

func sessionCSRFToken(request *http.Request) string {
	token, _ := services.SessionCSRFToken(request)
	return token
}
