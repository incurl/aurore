// Package minimax provides a typed client for the MiniMax Image-01
// (image_generation) API.
//
// The request/response shapes in this file mirror MiniMax's published
// OpenAPI specification for POST /v1/image_generation. Only fields that
// actually exist in that schema are modelled; unsupported diffusion
// concepts (CFG scale, samplers, steps, schedulers, LoRA, ControlNet,
// negative prompt, alpha mask) are deliberately absent.
package minimax

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Model identifies a MiniMax image model.
type Model string

const (
	// ModelImage01 is the standard image model. It supports aspect_ratio,
	// width/height, subject_reference (character) and no style presets.
	ModelImage01 Model = "image-01"

	// ModelImage01Live is the "live" image model. It supports the style
	// preset object but does NOT support aspect_ratio/width/height.
	ModelImage01Live Model = "image-01-live"
)

// AspectRatio is the MiniMax aspect ratio enum. It applies only to
// ModelImage01. When both aspect_ratio and width/height are supplied the
// API gives precedence to aspect_ratio.
type AspectRatio string

const (
	AspectRatio1x1  AspectRatio = "1:1"
	AspectRatio16x9 AspectRatio = "16:9"
	AspectRatio4x3  AspectRatio = "4:3"
	AspectRatio3x2  AspectRatio = "3:2"
	AspectRatio2x3  AspectRatio = "2:3"
	AspectRatio3x4  AspectRatio = "3:4"
	AspectRatio9x16 AspectRatio = "9:16"
	AspectRatio21x9 AspectRatio = "21:9"
)

// AllAspectRatios lists every aspect ratio accepted by the API, in the
// order used by the UI selector.
func AllAspectRatios() []AspectRatio {
	return []AspectRatio{
		AspectRatio1x1,
		AspectRatio16x9,
		AspectRatio4x3,
		AspectRatio3x2,
		AspectRatio2x3,
		AspectRatio3x4,
		AspectRatio9x16,
		AspectRatio21x9,
	}
}

// StyleType is a MiniMax style preset. Styles apply only to
// ModelImage01Live.
type StyleType string

const (
	StyleComic      StyleType = "漫画"  // comic
	StyleVitality   StyleType = "元气"  // vitality
	StyleMedieval   StyleType = "中世纪" // medieval
	StyleWatercolor StyleType = "水彩"  // watercolor
)

// AllStyleTypes lists every style preset accepted by the API.
func AllStyleTypes() []StyleType {
	return []StyleType{StyleComic, StyleVitality, StyleMedieval, StyleWatercolor}
}

// SubjectReferenceType is the kind of subject reference. The published
// schema documents only "character".
type SubjectReferenceType string

const (
	// SubjectReferenceCharacter is a person/character reference image.
	SubjectReferenceCharacter SubjectReferenceType = "character"
)

// ResponseFormat selects how generated images are returned.
type ResponseFormat string

const (
	// ResponseFormatURL returns expiring (24h) CDN URLs.
	ResponseFormatURL ResponseFormat = "url"
	// ResponseFormatBase64 returns inline base64 payloads.
	ResponseFormatBase64 ResponseFormat = "base64"
)

// StyleObject is the MiniMax style preset payload. It is only honoured
// when Model is ModelImage01Live.
type StyleObject struct {
	// StyleType is the preset name. Required when Style is present.
	StyleType StyleType `json:"style_type"`
	// StyleWeight is the style strength in the range (0, 1]. Defaults to
	// 0.8 server-side. Zero means "use the server default".
	StyleWeight float64 `json:"style_weight,omitempty"`
}

// SubjectReference is a reference image used for image-to-image
// generation.
type SubjectReference struct {
	// Type is the subject kind. Only "character" is documented.
	Type SubjectReferenceType `json:"type"`
	// ImageFile is the reference image as a public URL or a base64 data
	// URL. Desktop clients have no public URL, so Aurore sends a data
	// URL. Format must be JPG/JPEG/PNG and under 10 MB.
	ImageFile string `json:"image_file"`
}

// ImageRequest is the request body for POST /v1/image_generation.
//
// It is a superset of the API schema: it also carries the negative prompt
// and the mask asset, which Aurore resolves locally (see BuildPayload)
// because MiniMax has no such fields.
type ImageRequest struct {
	// Model is the target model. Required.
	Model Model `json:"model"`

	// Prompt is the positive prompt. Required, max 1500 characters.
	Prompt string `json:"prompt"`

	// NegativePrompt is NOT a MiniMax field. Aurore stores it on the job
	// and optionally folds it into Prompt as "Avoid: ...".
	NegativePrompt string `json:"negative_prompt,omitempty"`

	// SubjectReference is the optional reference image. Only one entry is
	// supported by the API.
	SubjectReference []SubjectReference `json:"subject_reference,omitempty"`

	// Style is the optional style preset. Only valid for ModelImage01Live.
	Style *StyleObject `json:"style,omitempty"`

	// AspectRatio selects the output ratio. Only valid for ModelImage01.
	AspectRatio AspectRatio `json:"aspect_ratio,omitempty"`

	// Width is the output width in pixels. Only valid for ModelImage01.
	// Must be set together with Height, in [512, 2048], multiple of 8.
	Width int `json:"width,omitempty"`

	// Height is the output height in pixels. Only valid for ModelImage01.
	// Must be set together with Width, in [512, 2048], multiple of 8.
	Height int `json:"height,omitempty"`

	// ResponseFormat selects URL or base64 output. Defaults to "url".
	ResponseFormat ResponseFormat `json:"response_format,omitempty"`

	// Seed is the reproducibility seed. Zero means "let the server pick".
	Seed int64 `json:"seed,omitempty"`

	// N is the number of images to generate, 1..9. Defaults to 1.
	N int `json:"n,omitempty"`

	// PromptOptimizer enables server-side prompt optimisation.
	PromptOptimizer bool `json:"prompt_optimizer,omitempty"`

	// AIGCWatermark adds the MiniMax AI-generated watermark.
	AIGCWatermark bool `json:"aigc_watermark,omitempty"`

	// MaskAssetID is NOT a MiniMax field. It references the uploaded
	// mask asset that Aurore composites locally onto the result.
	MaskAssetID string `json:"mask_asset_id,omitempty"`
}

// BaseResp is MiniMax's standard response envelope.
type BaseResp struct {
	// StatusCode is 0 on success. See the Status* constants for the
	// documented failure codes.
	StatusCode int `json:"status_code"`
	// StatusMsg is the human readable status or error detail.
	StatusMsg string `json:"status_msg"`
}

// UnmarshalJSON accepts the status code as a JSON number or as a
// numeric string.
//
// The published schema declares status_code as an integer, but the live
// API is not always consistent with it. Because a single mismatched
// field aborts the whole decode, a stringified status code would turn
// every error path into an "unreadable response", hiding the real
// cause of the failure.
func (b *BaseResp) UnmarshalJSON(data []byte) error {
	var raw struct {
		StatusCode json.RawMessage `json:"status_code"`
		StatusMsg  string          `json:"status_msg"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	code, err := parseCount(raw.StatusCode)
	if err != nil {
		return fmt.Errorf("minimax: base_resp.status_code: %w", err)
	}

	b.StatusCode = code
	b.StatusMsg = raw.StatusMsg
	return nil
}

// DataObject carries the generated images.
type DataObject struct {
	// ImageURLs is populated when ResponseFormat is "url".
	ImageURLs []string `json:"image_urls,omitempty"`
	// ImageBase64 is populated when ResponseFormat is "base64".
	ImageBase64 []string `json:"image_base64,omitempty"`
}

// Metadata reports per-request image counts.
type Metadata struct {
	// SuccessCount is the number of images actually generated.
	SuccessCount int `json:"success_count"`
	// FailedCount is the number of images rejected by content safety.
	FailedCount int `json:"failed_count"`
}

// UnmarshalJSON accepts the counts as JSON numbers or as numeric
// strings.
//
// MiniMax's live API returns "success_count":"3" and "failed_count":"0"
// as strings, even though the published schema declares them as
// integers. encoding/json refuses to put a string into an int field, and
// because one bad field aborts the whole decode, that mismatch used to
// fail the entire response: a successful generation was reported as
// "invalid parameters" and the images it had already produced were
// thrown away.
func (m *Metadata) UnmarshalJSON(data []byte) error {
	var raw struct {
		SuccessCount json.RawMessage `json:"success_count"`
		FailedCount  json.RawMessage `json:"failed_count"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	success, err := parseCount(raw.SuccessCount)
	if err != nil {
		return fmt.Errorf("minimax: metadata.success_count: %w", err)
	}
	failed, err := parseCount(raw.FailedCount)
	if err != nil {
		return fmt.Errorf("minimax: metadata.failed_count: %w", err)
	}

	m.SuccessCount = success
	m.FailedCount = failed
	return nil
}

// parseCount reads a count that may arrive as a JSON number or as a
// numeric string.
//
// A missing, null or empty value counts as zero, because the API omits
// the fields when there is nothing to report.
func parseCount(raw json.RawMessage) (int, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return 0, nil
	}

	// A quoted number, as MiniMax sends the counts.
	if strings.HasPrefix(trimmed, `"`) {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, fmt.Errorf("%s is not a number", trimmed)
		}
		text = strings.TrimSpace(text)
		if text == "" {
			return 0, nil
		}
		value, err := strconv.Atoi(text)
		if err != nil {
			return 0, fmt.Errorf("%q is not a number", text)
		}
		return value, nil
	}

	// A bare number.
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, fmt.Errorf("%s is not a number", trimmed)
	}
	return value, nil
}

// ImageResponse is the response body of POST /v1/image_generation.
//
// The API is synchronous: images are returned inline. There is no task
// query endpoint for image generation.
type ImageResponse struct {
	// Data holds the generated images.
	Data DataObject `json:"data"`
	// Metadata reports success/failure counts.
	Metadata Metadata `json:"metadata"`
	// ID is the generation task identifier, recorded for traceability.
	ID string `json:"id"`
	// BaseResp is the status envelope.
	BaseResp BaseResp `json:"base_resp"`
}

// Documented MiniMax status codes for the image generation API.
const (
	// StatusOK indicates success.
	StatusOK = 0
	// StatusRateLimited indicates the request was rate limited.
	StatusRateLimited = 1002
	// StatusAuthFailed indicates the API key failed authentication.
	StatusAuthFailed = 1004
	// StatusInsufficientBalance indicates the account is out of credit.
	StatusInsufficientBalance = 1008
	// StatusSensitiveContent indicates the prompt tripped content safety.
	StatusSensitiveContent = 1026
	// StatusInvalidParams indicates the request payload was malformed.
	StatusInvalidParams = 2013
	// StatusInvalidAPIKey indicates the API key is not valid.
	StatusInvalidAPIKey = 2049
)

// StatusUnreadableResponse is an Aurore-side code, not one of the codes
// MiniMax documents. It means the API replied, but the body could not be
// interpreted.
//
// It is deliberately not StatusInvalidParams: 2013 means MiniMax
// rejected the request parameters, and reporting it for a body this
// client could not parse sends the reader in the wrong direction, since
// the parameters may be perfectly fine.
const StatusUnreadableResponse = -1

// Error is a typed MiniMax API error.
type Error struct {
	// StatusCode is the MiniMax base_resp.status_code value.
	StatusCode int
	// Message is the human readable error text.
	Message string
	// HTTPStatus is the transport level HTTP status code.
	HTTPStatus int
	// Raw is the unparsed response body, when one was received. The
	// debug console shows it, so that an unexpected API response can be
	// inspected without adding a packet capture.
	Raw string
	// Err is the wrapped underlying error, if any.
	Err error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("minimax: %s (status %d): %v", e.Message, e.StatusCode, e.Err)
	}
	return fmt.Sprintf("minimax: %s (status %d)", e.Message, e.StatusCode)
}

// Unwrap exposes the underlying error to errors.Is/As.
func (e *Error) Unwrap() error { return e.Err }

// IsRateLimited reports whether the error is a rate limit rejection.
func (e *Error) IsRateLimited() bool { return e.StatusCode == StatusRateLimited }

// IsAuthError reports whether the error is an authentication or key
// validation failure.
func (e *Error) IsAuthError() bool {
	return e.StatusCode == StatusAuthFailed || e.StatusCode == StatusInvalidAPIKey
}

// IsInsufficientBalance reports whether the account lacks credit.
func (e *Error) IsInsufficientBalance() bool {
	return e.StatusCode == StatusInsufficientBalance
}

// IsContentPolicy reports whether the prompt was rejected by content
// safety.
func (e *Error) IsContentPolicy() bool {
	return e.StatusCode == StatusSensitiveContent
}

// IsInvalidParams reports whether the request payload was rejected.
func (e *Error) IsInvalidParams() bool {
	return e.StatusCode == StatusInvalidParams
}

// IsUnreadableResponse reports whether the response body could not be
// interpreted.
func (e *Error) IsUnreadableResponse() bool {
	return e.StatusCode == StatusUnreadableResponse
}

// HumanMessage returns a user-facing explanation of the error, suitable
// for display on a failed job record.
func (e *Error) HumanMessage() string {
	if e == nil {
		return ""
	}
	switch e.StatusCode {
	case StatusRateLimited:
		return "Rate limited by MiniMax (10 requests per minute). Please wait a moment and retry."
	case StatusAuthFailed:
		// 1004 is the login failure. Once whitespace and quotes are
		// stripped, the remaining causes are a key taken from the wrong
		// place, a revoked key, or a key and host from different
		// regions, which MiniMax does not mix.
		return "MiniMax rejected the API key (1004). Settings must hold the API secret key itself rather than the GroupId, the key must still be active, and it must belong to the same region as the API base URL."
	case StatusInvalidAPIKey:
		return "The MiniMax API key is invalid (2049). Check the credentials in Settings."
	case StatusInsufficientBalance:
		return "The MiniMax account has insufficient balance. Top up the account and retry."
	case StatusSensitiveContent:
		return "MiniMax blocked the prompt for sensitive content. Revise the prompt and retry."
	case StatusInvalidParams:
		return "MiniMax rejected the request parameters. Adjust the generation settings and retry."
	case StatusUnreadableResponse:
		return "MiniMax replied, but the response could not be read. This is not a parameter problem: the raw body is in the debug console, where it can be copied into a bug report."
	}
	if e.Message != "" {
		return e.Message
	}
	return "The MiniMax request failed."
}

// rawRequest is the wire representation of the request body. It exists so
// that unsupported concepts can never leak onto the wire even if a
// caller populates them.
type rawRequest struct {
	Model            Model              `json:"model"`
	Prompt           string             `json:"prompt"`
	SubjectReference []SubjectReference `json:"subject_reference,omitempty"`
	Style            *StyleObject       `json:"style,omitempty"`
	AspectRatio      AspectRatio        `json:"aspect_ratio,omitempty"`
	Width            int                `json:"width,omitempty"`
	Height           int                `json:"height,omitempty"`
	ResponseFormat   ResponseFormat     `json:"response_format,omitempty"`
	Seed             int64              `json:"seed,omitempty"`
	N                int                `json:"n,omitempty"`
	PromptOptimizer  bool               `json:"prompt_optimizer,omitempty"`
	AIGCWatermark    bool               `json:"aigc_watermark,omitempty"`
}

// MarshalJSON implements json.Marshaler. It is the last line of defence
// ensuring that negative prompts and mask assets are never transmitted to
// MiniMax, which have no such fields.
func (r ImageRequest) MarshalJSON() ([]byte, error) {
	raw := rawRequest{
		Model:            r.Model,
		Prompt:           r.Prompt,
		SubjectReference: r.SubjectReference,
		Style:            r.Style,
		AspectRatio:      r.AspectRatio,
		Width:            r.Width,
		Height:           r.Height,
		ResponseFormat:   r.ResponseFormat,
		Seed:             r.Seed,
		N:                r.N,
		PromptOptimizer:  r.PromptOptimizer,
		AIGCWatermark:    r.AIGCWatermark,
	}
	return json.Marshal(raw)
}
