package minimax

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the documented MiniMax API host.
const DefaultBaseURL = "https://api.minimax.cn"

// ImageGenerationPath is the documented endpoint for text-to-image and
// image-to-image generation.
const ImageGenerationPath = "/v1/image_generation"

// DefaultTimeout bounds a single generation request.
const DefaultTimeout = 120 * time.Second

// Client is a typed HTTP client for the MiniMax image generation API.
//
// The API is synchronous: a single POST returns the generated images
// inline. There is no task query or polling endpoint for image
// generation.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	limiter    *RateLimiter
	sem        chan struct{}
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithBaseURL overrides the API host. Used in tests.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		// A pasted host carries the same stray whitespace as a pasted
		// key, and a trailing slash would double up with the path.
		c.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	}
}

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.httpClient = hc }
}

// WithRateLimiter overrides the rate limiter.
func WithRateLimiter(l *RateLimiter) ClientOption {
	return func(c *Client) { c.limiter = l }
}

// WithMaxConcurrent overrides the in-flight request limit.
func WithMaxConcurrent(n int) ClientOption {
	return func(c *Client) {
		if n <= 0 {
			n = DefaultMaxConcurrent
		}
		c.sem = make(chan struct{}, n)
	}
}

// NewClient creates a MiniMax API client.
//
// apiKey is the MiniMax API key sent as a bearer token. It is normalised
// first, so a key copied with surrounding whitespace, quotes or the whole
// "Bearer ..." header still authenticates.
func NewClient(apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		apiKey:     SanitizeAPIKey(apiKey),
		httpClient: &http.Client{Timeout: DefaultTimeout},
		limiter:    NewDefaultRateLimiter(),
		sem:        make(chan struct{}, DefaultMaxConcurrent),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}
	return c
}

// GenerateResult holds the decoded outcome of a successful generation.
type GenerateResult struct {
	// TaskID is the MiniMax generation task identifier.
	TaskID string
	// Images holds the decoded image bytes, in generation order.
	Images [][]byte
	// SuccessCount is the number of images MiniMax actually produced.
	SuccessCount int
	// FailedCount is the number of images rejected by content safety.
	FailedCount int
}

// Generate performs a synchronous image generation request.
//
// It waits for a rate limit token, bounds concurrency, posts the request
// and decodes the response. Any non-success response is returned as an
// *Error carrying the MiniMax status code.
func (c *Client) Generate(ctx context.Context, req ImageRequest) (*GenerateResult, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, &Error{
			StatusCode: StatusAuthFailed,
			Message:    "no MiniMax API key configured",
		}
	}

	payload, err := BuildPayload(req)
	if err != nil {
		return nil, &Error{StatusCode: StatusInvalidParams, Message: err.Error(), Err: err}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &Error{StatusCode: StatusInvalidParams, Message: "failed to encode request", Err: err}
	}

	// Respect the documented 10 RPM limit before issuing the request.
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, &Error{StatusCode: StatusRateLimited, Message: "rate limiter aborted", Err: err}
	}

	// Bound the number of concurrent in-flight generations.
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, &Error{StatusCode: StatusRateLimited, Message: "request cancelled", Err: ctx.Err()}
	}

	url := c.baseURL + ImageGenerationPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, &Error{StatusCode: StatusInvalidParams, Message: "failed to build request", Err: err}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, &Error{
			StatusCode: StatusInvalidParams,
			Message:    "could not reach the MiniMax API",
			Err:        err,
		}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &Error{
			StatusCode: StatusInvalidParams,
			Message:    "failed to read the MiniMax response",
			HTTPStatus: resp.StatusCode,
			Err:        err,
		}
	}

	var parsed ImageResponse
	decodeErr := json.Unmarshal(raw, &parsed)

	// The raw body is kept so that the debug console can show what the
	// API actually returned.
	rawBody := string(raw)

	// A non-2xx HTTP status is a transport/auth failure even if the body
	// happens to decode.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(http.StatusText(resp.StatusCode))
		if decodeErr == nil && parsed.BaseResp.StatusMsg != "" {
			msg = parsed.BaseResp.StatusMsg
		}
		status := StatusAuthFailed
		if resp.StatusCode == http.StatusTooManyRequests {
			status = StatusRateLimited
		}
		return nil, &Error{
			StatusCode: status,
			Message:    msg,
			HTTPStatus: resp.StatusCode,
			Raw:        rawBody,
		}
	}

	if decodeErr != nil {
		return nil, &Error{
			StatusCode: StatusInvalidParams,
			Message:    "the MiniMax response could not be decoded",
			HTTPStatus: resp.StatusCode,
			Raw:        rawBody,
			Err:        decodeErr,
		}
	}

	if parsed.BaseResp.StatusCode != StatusOK {
		return nil, &Error{
			StatusCode: parsed.BaseResp.StatusCode,
			Message:    parsed.BaseResp.StatusMsg,
			HTTPStatus: resp.StatusCode,
			Raw:        rawBody,
		}
	}

	result := &GenerateResult{
		TaskID:       parsed.ID,
		SuccessCount: parsed.Metadata.SuccessCount,
		FailedCount:  parsed.Metadata.FailedCount,
	}

	if len(parsed.Data.ImageBase64) > 0 {
		for i, encoded := range parsed.Data.ImageBase64 {
			decoded, decErr := base64.StdEncoding.DecodeString(encoded)
			if decErr != nil {
				return nil, &Error{
					StatusCode: StatusInvalidParams,
					Message:    fmt.Sprintf("image %d could not be decoded", i),
					HTTPStatus: resp.StatusCode,
					Err:        decErr,
				}
			}
			result.Images = append(result.Images, decoded)
		}
		return result, nil
	}

	if len(parsed.Data.ImageURLs) > 0 {
		for i, imageURL := range parsed.Data.ImageURLs {
			data, dlErr := c.downloadImage(ctx, imageURL)
			if dlErr != nil {
				return nil, &Error{
					StatusCode: StatusInvalidParams,
					Message:    fmt.Sprintf("image %d could not be downloaded", i),
					HTTPStatus: resp.StatusCode,
					Err:        dlErr,
				}
			}
			result.Images = append(result.Images, data)
		}
		return result, nil
	}

	return nil, &Error{
		StatusCode: StatusInvalidParams,
		Message:    "MiniMax returned no images",
		HTTPStatus: resp.StatusCode,
	}
}

// downloadImage fetches a generated image from a MiniMax CDN URL. These
// URLs expire 24 hours after generation, so Aurore always downloads the
// bytes and stores them locally.
func (c *Client) downloadImage(ctx context.Context, imageURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d downloading image", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// Ping verifies that the configured API key is accepted by MiniMax.
//
// The image generation endpoint has no dedicated health check, so this
// issues a deliberately empty prompt. A response whose base_resp reports
// an authentication problem means the key is bad; a parameter error means
// the key was accepted. This avoids generating an image (and therefore
// avoids spending quota) merely to test credentials.
func (c *Client) Ping(ctx context.Context) error {
	if strings.TrimSpace(c.apiKey) == "" {
		return &Error{StatusCode: StatusAuthFailed, Message: "no MiniMax API key configured"}
	}

	payload := map[string]any{"model": string(ModelImage01), "prompt": ""}
	body, err := json.Marshal(payload)
	if err != nil {
		return &Error{StatusCode: StatusInvalidParams, Message: "failed to encode request", Err: err}
	}

	url := c.baseURL + ImageGenerationPath
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return &Error{StatusCode: StatusInvalidParams, Message: "failed to build request", Err: err}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return &Error{Message: "could not reach the MiniMax API", Err: err}
	}
	defer resp.Body.Close()

	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return &Error{Message: "failed to read the MiniMax response", Err: readErr}
	}

	var parsed ImageResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return &Error{
			Message: "the MiniMax response could not be decoded",
			Raw:     string(raw),
			Err:     err,
		}
	}

	switch parsed.BaseResp.StatusCode {
	case StatusOK, StatusInvalidParams:
		// The key was accepted; the empty prompt was simply invalid.
		return nil
	default:
		return &Error{
			StatusCode: parsed.BaseResp.StatusCode,
			Message:    parsed.BaseResp.StatusMsg,
			HTTPStatus: resp.StatusCode,
			Raw:        string(raw),
		}
	}
}
