package services

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestResendEmailSenderSendsTransactionalEmailContract(t *testing.T) {
	const apiKey = "test-resend-secret"
	var received struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text"`
		HTML    string   `json:"html"`
	}

	sender := NewResendEmailSender(apiKey, "Universal Curriculum <no-reply@curriculum.example>")
	sender.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.String() != resendEmailsEndpoint {
			t.Fatalf("request = %s %s", request.Method, request.URL)
		}
		if request.Header.Get("Authorization") != "Bearer "+apiKey {
			t.Fatal("missing Resend bearer authentication")
		}
		if request.Header.Get("User-Agent") != "universal-curriculum" {
			t.Fatalf("User-Agent = %q", request.Header.Get("User-Agent"))
		}
		if request.Header.Get("Idempotency-Key") != "password-reset/message-1" {
			t.Fatalf("idempotency key = %q", request.Header.Get("Idempotency-Key"))
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"email-id"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	err := sender.Send(context.Background(), EmailMessage{
		To:             "learner@example.test",
		Subject:        "Subject",
		Text:           "Plain text",
		HTML:           "<p>HTML</p>",
		IdempotencyKey: "password-reset/message-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if received.From != "Universal Curriculum <no-reply@curriculum.example>" ||
		len(received.To) != 1 || received.To[0] != "learner@example.test" ||
		received.Subject != "Subject" || received.Text == "" || received.HTML == "" {
		t.Fatalf("Resend payload = %+v", received)
	}
}

func TestResendEmailSenderReturnsProviderFailureWithoutExposingAPIKey(t *testing.T) {
	const apiKey = "test-resend-secret"
	sender := NewResendEmailSender(apiKey, "Universal Curriculum <no-reply@curriculum.example>")
	sender.client = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnprocessableEntity,
			Body:       io.NopCloser(strings.NewReader(`{"message":"sender domain is not verified"}`)),
			Header:     make(http.Header),
		}, nil
	})}

	err := sender.Send(context.Background(), EmailMessage{
		To:      "learner@example.test",
		Subject: "Subject",
		Text:    "Content",
	})
	if err == nil || !strings.Contains(err.Error(), "sender domain is not verified") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), apiKey) {
		t.Fatal("provider error exposes API key")
	}
}
