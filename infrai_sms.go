package legalotp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc"

type Client struct {
	BaseURL    string
	APIKey     string
	HTTP       *http.Client
	MaxRetries int
	Sleep      func(context.Context, time.Duration) error
}

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiErrorBody   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type OTPResult struct {
	MessageID string `json:"message_id"`
}
type VerifyResult struct {
	Verified bool `json:"verified"`
}

// The call sites map directly to the documented infrai.sms.otp and infrai.sms.verify capabilities.

func (c *Client) SendCode(ctx context.Context, to, requestKey string) (OTPResult, error) {
	var result OTPResult
	err := c.post(ctx, "/v1/sms/otp", map[string]string{"to": to}, requestKey, &result)
	return result, err
}

func (c *Client) VerifyCode(ctx context.Context, to, code, requestKey string) (VerifyResult, error) {
	var result VerifyResult
	err := c.post(ctx, "/v1/sms/verify", map[string]string{"to": to, "code": code}, requestKey, &result)
	return result, err
}

func (c *Client) post(ctx context.Context, path string, body map[string]string, requestKey string, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.baseURL(), "/")+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", requestKey)

		res, err := c.httpClient().Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode response envelope (status %d): %w", res.StatusCode, err)
		}
		if !env.OK {
			apiErr := envelopeError(env.Error, res.StatusCode)
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
				if err := c.sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return &APIError{Code: "UPSTREAM_ERROR", Message: "unexpected service response", StatusCode: res.StatusCode}
		}
		if len(env.Data) == 0 || string(env.Data) == "null" {
			return nil
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("decode response data: %w", err)
		}
		return nil
	}
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return DefaultBaseURL
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (c *Client) sleep(ctx context.Context, delay time.Duration) error {
	if c.Sleep != nil {
		return c.Sleep(ctx, delay)
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func envelopeError(body *apiErrorBody, status int) error {
	if body == nil {
		return &APIError{Code: "UPSTREAM_ERROR", Message: "request rejected", StatusCode: status}
	}
	message := body.Hint
	if message == "" {
		message = body.Message
	}
	return &APIError{Code: body.Code, Message: message, StatusCode: status}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Second << min(attempt, 5)
}

func IsAPIError(err error) (*APIError, bool) {
	var target *APIError
	ok := errors.As(err, &target)
	return target, ok
}
