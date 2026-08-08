package services

import "testing"

func TestValidPKCEVerifier(t *testing.T) {
	valid := "0123456789abcdefghijklmnopqrstuvwxyz-._~ABC"
	if !validPKCEVerifier(valid) {
		t.Fatalf("valid verifier was rejected")
	}
	for _, invalid := range []string{"short", valid + "!", string(make([]byte, 129))} {
		if validPKCEVerifier(invalid) {
			t.Fatalf("invalid verifier was accepted: %q", invalid)
		}
	}
}
