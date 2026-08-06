package db

import "testing"

func TestAPITokenValidationAndHashing(t *testing.T) {
	valid := "uc_api_1234567890123456789012345678901234567890123"
	if !validAPIToken(valid) {
		t.Fatal("valid API token was rejected")
	}
	for _, token := range []string{
		"", "1234567890123456789012345678901234567890123",
		"uc_api_short", valid + "x", "uc_api_123456789012345678901234567890123456789012!",
	} {
		if validAPIToken(token) {
			t.Fatalf("invalid API token %q was accepted", token)
		}
	}
	if hashAPIToken(valid) == valid || len(hashAPIToken(valid)) != 64 {
		t.Fatal("API token is not represented by a SHA-256 hex hash")
	}
}
