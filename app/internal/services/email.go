package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const resendEmailsEndpoint = "https://api.resend.com/emails"

type EmailMessage struct {
	To             string
	Subject        string
	Text           string
	HTML           string
	IdempotencyKey string
}

type EmailSender interface {
	Send(context.Context, EmailMessage) error
}

type ResendEmailSender struct {
	apiKey   string
	from     string
	endpoint string
	client   *http.Client
}

func NewResendEmailSender(apiKey, from string) *ResendEmailSender {
	return &ResendEmailSender{
		apiKey:   apiKey,
		from:     from,
		endpoint: resendEmailsEndpoint,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (sender *ResendEmailSender) Send(ctx context.Context, message EmailMessage) error {
	if sender == nil || sender.apiKey == "" || sender.from == "" {
		return errors.New("email sender is not configured")
	}
	if strings.TrimSpace(message.To) == "" || strings.TrimSpace(message.Subject) == "" ||
		(message.Text == "" && message.HTML == "") {
		return errors.New("email recipient, subject and content are required")
	}

	payload, err := json.Marshal(struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		Text    string   `json:"text,omitempty"`
		HTML    string   `json:"html,omitempty"`
	}{
		From:    sender.from,
		To:      []string{message.To},
		Subject: message.Subject,
		Text:    message.Text,
		HTML:    message.HTML,
	})
	if err != nil {
		return fmt.Errorf("encode email request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, sender.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create email request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+sender.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "universal-curriculum")
	if message.IdempotencyKey != "" {
		request.Header.Set("Idempotency-Key", message.IdempotencyKey)
	}

	response, err := sender.client.Do(request)
	if err != nil {
		return fmt.Errorf("send email through Resend: %w", err)
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 32*1024))
	if err != nil {
		return fmt.Errorf("read Resend response: %w", err)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var apiError struct {
			Message string `json:"message"`
		}
		if json.Unmarshal(responseBody, &apiError) == nil && apiError.Message != "" {
			return fmt.Errorf("Resend rejected email with status %d: %s", response.StatusCode, apiError.Message)
		}
		return fmt.Errorf("Resend rejected email with status %d", response.StatusCode)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(responseBody, &result); err != nil || result.ID == "" {
		return errors.New("Resend returned an invalid success response")
	}
	return nil
}
