package services

import (
	"errors"
	"strings"
	"testing"
)

func TestCreateAPITokenValidatesNameBeforePersistence(t *testing.T) {
	if _, err := CreateAPIToken(nil, 1, "  "); !errors.Is(err, ErrAPITokenNameRequired) {
		t.Fatalf("empty name error = %v", err)
	}
	if _, err := CreateAPIToken(nil, 1, strings.Repeat("a", MaximumAPITokenNameLength+1)); !errors.Is(err, ErrAPITokenNameTooLong) {
		t.Fatalf("long name error = %v", err)
	}
}
