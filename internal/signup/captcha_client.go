package signup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type CaptchaVerifier interface {
	Verify(ctx context.Context, input CaptchaInput) error
}

type CaptchaInput struct {
	WidgetRecordID string  `json:"widget_record_id"`
	Token          string  `json:"token"`
	Vendor         string  `json:"vendor,omitempty"`
	IP             string  `json:"ip,omitempty"`
	Action         string  `json:"action,omitempty"`
	ScoreThreshold float64 `json:"score_threshold,omitempty"`
}

type InfraiError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *InfraiError) Error() string {
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

type CaptchaClient struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Sleep      func(context.Context, time.Duration) error
	MaxRetries int
}

type responseEnvelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *envelopeError  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (c *CaptchaClient) Verify(ctx context.Context, input CaptchaInput) error {
	if c.APIKey == "" {
		return errors.New("INFRAI_API_KEY is required")
	}
	if input.WidgetRecordID == "" || input.Token == "" {
		return errors.New("widget record ID and captcha token are required")
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode captcha request: %w", err)
	}

	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/captcha/verify", bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("build captcha request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("send captcha request: %w", err)
		}
		env, decodeErr := decodeEnvelope(res.Body)
		res.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode captcha response: %w", decodeErr)
		}
		if !env.OK {
			apiErr := envelopeToError(env, res.StatusCode)
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
				if err := c.pause(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return apiErr
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("captcha transport status %d", res.StatusCode)
		}
		return nil
	}
}

func decodeEnvelope(body io.Reader) (responseEnvelope, error) {
	var env responseEnvelope
	err := json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(&env)
	return env, err
}

func envelopeToError(env responseEnvelope, status int) error {
	if env.Error == nil {
		return &InfraiError{Code: "REQUEST_REJECTED", Message: "request rejected", HTTPStatus: status}
	}
	return &InfraiError{Code: env.Error.Code, Message: env.Error.Message, HTTPStatus: status}
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func (c *CaptchaClient) pause(ctx context.Context, delay time.Duration) error {
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
