package minimax

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// H3Timeout bounds a single H3 request.
//
// The H3 server waits up to 600 seconds before giving up, and the
// documentation asks clients to allow at least that long. A shorter
// client timeout would abandon a request that is still being generated
// and is still billed.
const H3Timeout = 620 * time.Second

// generateH3 performs an H3 text-to-image or image-edit request.
func (c *Client) generateH3(ctx context.Context, req ImageRequest) (*GenerateResult, error) {
	path, payload, err := BuildH3Payload(req)
	if err != nil {
		return nil, &Error{StatusCode: StatusInvalidParams, Message: err.Error(), Err: err}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &Error{StatusCode: StatusInvalidParams, Message: "failed to encode request", Err: err}
	}

	// H3 is limited in images per minute, so a batch spends one token per
	// image rather than one per request.
	n := req.N
	if n <= 0 {
		n = 1
	}
	if err := c.h3Limiter.WaitN(ctx, n); err != nil {
		return nil, &Error{StatusCode: StatusRateLimited, Message: "rate limiter aborted", Err: err}
	}

	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, &Error{StatusCode: StatusRateLimited, Message: "request cancelled", Err: ctx.Err()}
	}

	// Bound the request by the H3 timeout without lowering the shared
	// client timeout, which the Image-01 path still relies on.
	ctx, cancel := context.WithTimeout(ctx, H3Timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
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

	// An error response is reported in the OpenAI-style envelope. It can
	// accompany a 2xx as well, when every image failed.
	var failure h3ErrorEnvelope
	if err := json.Unmarshal(raw, &failure); err == nil && failure.Error.Message != "" {
		return nil, &Error{
			StatusCode: failure.statusCode(),
			Message:    failure.Error.Message,
			HTTPStatus: resp.StatusCode,
			Raw:        string(raw),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &Error{
			StatusCode: StatusAuthFailed,
			Message:    strings.TrimSpace(http.StatusText(resp.StatusCode)),
			HTTPStatus: resp.StatusCode,
			Raw:        string(raw),
		}
	}

	var parsed h3Response
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, decodeError(raw, resp.StatusCode, err)
	}

	result := &GenerateResult{
		TaskID:       parsed.RequestID,
		SuccessCount: parsed.Usage.GeneratedImages,
	}

	// H3 answers 200 when at least one image succeeded, and marks the
	// ones that did not with an error instead of an image.
	firstCode := 0
	for i, image := range parsed.Data {
		if image.Error != nil {
			result.FailedCount++
			if firstCode == 0 {
				firstCode = parseStatusCode(image.Error.Code)
			}
			continue
		}
		data, decErr := decodeH3Image(ctx, c, image)
		if decErr != nil {
			return nil, &Error{
				StatusCode: StatusInvalidParams,
				Message:    fmt.Sprintf("image %d could not be decoded", i),
				HTTPStatus: resp.StatusCode,
				Err:        decErr,
			}
		}
		result.Images = append(result.Images, data)
	}

	if len(result.Images) == 0 {
		// Report the code MiniMax gave for the first image, so a safety
		// refusal reads as one rather than as an unknown failure.
		status := StatusSensitiveContent
		message := "MiniMax returned no images: every image was rejected"
		if firstCode != 0 {
			status = firstCode
		}
		return nil, &Error{
			StatusCode: status,
			Message:    message,
			HTTPStatus: resp.StatusCode,
			Raw:        string(raw),
		}
	}

	// The count is reported by the server, but trust the images actually
	// decoded, so a mismatch cannot leave the job thinking it has more.
	if result.SuccessCount == 0 {
		result.SuccessCount = len(result.Images)
	}

	return result, nil
}

// pingH3 verifies the API key against the H3 endpoint.
//
// H3 rejects an empty prompt with 400 and a parameter code, which is
// exactly what an accepted key looks like here: the point is only to
// learn whether the credentials are the thing being refused.
func (c *Client) pingH3(ctx context.Context, model Model) error {
	payload := map[string]any{"model": string(model), "prompt": ""}
	body, err := json.Marshal(payload)
	if err != nil {
		return &Error{StatusCode: StatusInvalidParams, Message: "failed to encode request", Err: err}
	}

	ctx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+H3GenerationsPath, bytes.NewReader(body))
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

	var failure h3ErrorEnvelope
	if err := json.Unmarshal(raw, &failure); err == nil && failure.Error.Message != "" {
		code := failure.statusCode()
		if code == StatusInvalidParams || code == StatusOK ||
			code == StatusSensitiveContent {
			// The key was accepted; the empty prompt was simply invalid.
			return nil
		}
		return &Error{
			StatusCode: code,
			Message:    failure.Error.Message,
			HTTPStatus: resp.StatusCode,
			Raw:        string(raw),
		}
	}

	// A 2xx with no error envelope also means the request was authorised.
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return &Error{
		StatusCode: StatusAuthFailed,
		Message:    strings.TrimSpace(http.StatusText(resp.StatusCode)),
		HTTPStatus: resp.StatusCode,
		Raw:        string(raw),
	}
}

// parseStatusCode converts a code H3 reports as a string into the numeric
// form the rest of Aurore uses. An unparseable code becomes zero, which
// the callers treat as "not reported".
func parseStatusCode(raw string) int {
	code, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0
	}
	return code
}

// Aurore always asks for b64_json, but the URL form is accepted as well,
// so that a response shaped differently from the request still works.
func decodeH3Image(ctx context.Context, c *Client, image h3ImageObject) ([]byte, error) {
	if image.B64JSON != "" {
		data, err := base64.StdEncoding.DecodeString(image.B64JSON)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	if image.URL != "" {
		return c.downloadImage(ctx, image.URL)
	}
	return nil, fmt.Errorf("the image carried neither b64_json nor a url")
}
