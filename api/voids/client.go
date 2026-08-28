// Package voids provides a client for the third-party Voids quote API.
// It is independent from the local renderer in the root makeitaquote package.
package voids

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf16"

	miq "github.com/tikipiya/MiQ"
	internalnet "github.com/tikipiya/MiQ/internal/netpolicy"
)

const (
	DefaultBaseURL = "https://api.voids.top"
	HostedEndpoint = "/fakequote"
	DirectEndpoint = "/fakequotebeta"
)

var retryStatuses = map[int]bool{408: true, 413: true, 429: true, 500: true, 502: true, 503: true, 504: true}

// Quote is the wire-compatible input accepted by Voids. Unlike local
// rendering, the avatar must already be an HTTP(S) URL.
type Quote struct {
	Text        string
	Avatar      *url.URL
	Username    string
	DisplayName string
	Color       bool
	Watermark   string
}

type Options struct {
	BaseURL             *url.URL
	HTTPClient          *http.Client
	Timeout             time.Duration
	Retries             *int
	RetryDelay          *time.Duration
	Headers             http.Header
	MaxResponseBytes    int64
	AllowPrivateNetwork bool
}

type Client struct {
	baseURL             *url.URL
	httpClient          *http.Client
	timeout             time.Duration
	retries             int
	retryDelay          time.Duration
	headers             http.Header
	maxResponseBytes    int64
	allowPrivateNetwork bool
}

type payload struct {
	Text        string  `json:"text"`
	Avatar      *string `json:"avatar"`
	Username    string  `json:"username"`
	DisplayName string  `json:"display_name"`
	Color       bool    `json:"color"`
	Watermark   string  `json:"watermark"`
}

// APIError contains a failed endpoint's status and bounded response body.
type APIError struct {
	Endpoint string
	Status   int
	Body     []byte
	Err      error
}

func (e *APIError) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("Voids %s: HTTP %d: %v", e.Endpoint, e.Status, e.Err)
	}
	return fmt.Sprintf("Voids %s: %v", e.Endpoint, e.Err)
}
func (e *APIError) Unwrap() []error {
	if e.Err == nil {
		return []error{miq.ErrAPI}
	}
	return []error{miq.ErrAPI, e.Err}
}

func NewClient(options Options) (*Client, error) {
	base := options.BaseURL
	if base == nil {
		parsed, err := url.Parse(DefaultBaseURL)
		if err != nil {
			return nil, err
		}
		base = parsed
	} else {
		copyOfBase := *base
		base = &copyOfBase
	}
	if base.Scheme != "http" && base.Scheme != "https" {
		return nil, &miq.FieldError{Field: "baseURL", Err: fmt.Errorf("scheme must be http or https: %w", miq.ErrValidation)}
	}
	if base.Host == "" || base.RawQuery != "" || base.Fragment != "" {
		return nil, &miq.FieldError{Field: "baseURL", Err: fmt.Errorf("must be an absolute URL without query or fragment: %w", miq.ErrValidation)}
	}
	base.Path = strings.TrimRight(base.Path, "/")
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	timeout := options.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	if timeout < 0 {
		return nil, &miq.FieldError{Field: "timeout", Err: fmt.Errorf("must not be negative: %w", miq.ErrValidation)}
	}
	retries := 2
	if options.Retries != nil {
		retries = *options.Retries
	}
	if retries < 0 {
		return nil, &miq.FieldError{Field: "retries", Err: fmt.Errorf("must not be negative: %w", miq.ErrValidation)}
	}
	delay := 300 * time.Millisecond
	if options.RetryDelay != nil {
		delay = *options.RetryDelay
	}
	if delay < 0 {
		return nil, &miq.FieldError{Field: "retryDelay", Err: fmt.Errorf("must not be negative: %w", miq.ErrValidation)}
	}
	limit := options.MaxResponseBytes
	if limit == 0 {
		limit = 32 << 20
	}
	if limit < 1 {
		return nil, &miq.FieldError{Field: "maxResponseBytes", Err: fmt.Errorf("must be positive: %w", miq.ErrValidation)}
	}
	headers := options.Headers.Clone()
	if headers == nil {
		headers = make(http.Header)
	}
	if headers.Get("User-Agent") == "" {
		headers.Set("User-Agent", "makeitaquote-go/1")
	}
	return &Client{baseURL: base, httpClient: client, timeout: timeout, retries: retries, retryDelay: delay, headers: headers, maxResponseBytes: limit, allowPrivateNetwork: options.AllowPrivateNetwork}, nil
}

// HostedURL renders through /fakequote and returns the API-hosted image URL.
func (c *Client) HostedURL(ctx context.Context, quote Quote) (*url.URL, error) {
	body, err := c.post(ctx, HostedEndpoint, quote)
	if err != nil {
		return nil, err
	}
	var response struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, &APIError{Endpoint: HostedEndpoint, Body: append([]byte(nil), body...), Err: fmt.Errorf("response is not JSON: %w", err)}
	}
	result, err := url.Parse(response.URL)
	if err != nil || result.Scheme != "http" && result.Scheme != "https" || result.Host == "" {
		if err == nil {
			err = errors.New("response did not contain an absolute HTTP(S) url")
		}
		return nil, &APIError{Endpoint: HostedEndpoint, Body: append([]byte(nil), body...), Err: err}
	}
	return result, nil
}

// Direct renders through /fakequotebeta and returns the image bytes in one request.
func (c *Client) Direct(ctx context.Context, quote Quote) ([]byte, error) {
	return c.post(ctx, DirectEndpoint, quote)
}

// HostedBytes creates a hosted image and then downloads it in a second request.
func (c *Client) HostedBytes(ctx context.Context, quote Quote) ([]byte, error) {
	address, err := c.HostedURL(ctx, quote)
	if err != nil {
		return nil, err
	}
	return c.request(ctx, http.MethodGet, address, HostedEndpoint, nil)
}

func (c *Client) post(ctx context.Context, endpoint string, quote Quote) ([]byte, error) {
	if err := validateQuote(quote); err != nil {
		return nil, err
	}
	wire := payload{Text: quote.Text, Username: quote.Username, DisplayName: quote.DisplayName, Color: quote.Color, Watermark: quote.Watermark}
	if quote.Avatar != nil {
		value := quote.Avatar.String()
		wire.Avatar = &value
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, &APIError{Endpoint: endpoint, Err: err}
	}
	address := *c.baseURL
	address.Path = strings.TrimRight(c.baseURL.Path, "/") + endpoint
	return c.request(ctx, http.MethodPost, &address, endpoint, encoded)
}

func (c *Client) request(ctx context.Context, method string, address *url.URL, endpoint string, body []byte) ([]byte, error) {
	if ctx == nil {
		return nil, &miq.FieldError{Field: "context", Err: fmt.Errorf("must not be nil: %w", miq.ErrValidation)}
	}
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	for attempt := 0; attempt <= c.retries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := internalnet.Validate(ctx, address, c.allowPrivateNetwork); err != nil {
			return nil, &APIError{Endpoint: endpoint, Err: err}
		}
		req, err := http.NewRequestWithContext(ctx, method, address.String(), bytes.NewReader(body))
		if err != nil {
			return nil, &APIError{Endpoint: endpoint, Err: err}
		}
		req.Header = c.headers.Clone()
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attempt < c.retries {
				if err := wait(ctx, c.retryDelay); err != nil {
					return nil, err
				}
				continue
			}
			return nil, &APIError{Endpoint: endpoint, Err: err}
		}
		if resp.Request != nil && resp.Request.URL != nil {
			if policyErr := internalnet.Validate(ctx, resp.Request.URL, c.allowPrivateNetwork); policyErr != nil {
				resp.Body.Close()
				return nil, &APIError{Endpoint: endpoint, Err: fmt.Errorf("redirect target: %w", policyErr)}
			}
		}
		data, readErr := readBounded(resp.Body, c.maxResponseBytes)
		resp.Body.Close()
		if readErr != nil {
			return nil, &APIError{Endpoint: endpoint, Status: resp.StatusCode, Err: readErr}
		}
		if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
			return data, nil
		}
		apiErr := &APIError{Endpoint: endpoint, Status: resp.StatusCode, Body: data, Err: miq.ErrAPI}
		if attempt < c.retries && retryStatuses[resp.StatusCode] {
			if err := wait(ctx, c.retryDelay); err != nil {
				return nil, err
			}
			continue
		}
		return nil, apiErr
	}
	return nil, &APIError{Endpoint: endpoint, Err: miq.ErrAPI}
}

func validateQuote(quote Quote) error {
	if strings.TrimSpace(quote.Text) == "" {
		return &miq.FieldError{Field: "text", Err: fmt.Errorf("is required: %w", miq.ErrValidation)}
	}
	for _, field := range []struct {
		name, value string
		max         int
	}{{"text", quote.Text, miq.MaxTextLength}, {"username", quote.Username, miq.MaxNameLength}, {"displayName", quote.DisplayName, miq.MaxNameLength}, {"watermark", quote.Watermark, miq.MaxWatermarkLength}} {
		if len(utf16.Encode([]rune(field.value))) > field.max {
			return &miq.FieldError{Field: field.name, Err: fmt.Errorf("must be at most %d UTF-16 code units: %w", field.max, miq.ErrValidation)}
		}
	}
	if quote.Avatar != nil && (quote.Avatar.Scheme != "http" && quote.Avatar.Scheme != "https" || quote.Avatar.Host == "") {
		return &miq.FieldError{Field: "avatar", Err: fmt.Errorf("must be an absolute HTTP(S) URL: %w", miq.ErrValidation)}
	}
	return nil
}
func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}
func wait(ctx context.Context, duration time.Duration) error {
	if duration == 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
