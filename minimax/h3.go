package minimax

import (
	"fmt"
	"strconv"
	"strings"
)

// The H3 endpoints. They are a different API from
// POST /v1/image_generation: text-to-image and image editing are
// separate paths, and both accept MiniMax-H3-Image and
// MiniMax-H3-Image-Highspeed.
const (
	// H3GenerationsPath generates images from a prompt.
	H3GenerationsPath = "/v1/images/generations"
	// H3EditsPath generates images from reference images and a prompt.
	H3EditsPath = "/v1/images/edits"
)

// h3ImageInput is one reference image on the edit endpoint.
type h3ImageInput struct {
	// ImageURL is a public URL, a mm_file:// reference, or a
	// data:image/...;base64,... data URI. Aurore sends a data URI,
	// because a desktop client has no public URL to offer.
	ImageURL string `json:"image_url"`
}

// h3Request is the wire body for both H3 endpoints.
//
// It is deliberately narrow: H3 has no seed, no style object and no
// watermark flag, so those fields cannot be represented here at all.
type h3Request struct {
	Model             Model          `json:"model"`
	Prompt            string         `json:"prompt"`
	Images            []h3ImageInput `json:"images,omitempty"`
	DisablePromptEnhd bool           `json:"disable_prompt_enhanced,omitempty"`
	N                 int            `json:"n,omitempty"`
	AspectRatio       AspectRatio    `json:"aspect_ratio,omitempty"`
	Area              Area           `json:"area,omitempty"`
	OutputFormat      string         `json:"output_format,omitempty"`
	ResponseFormat    string         `json:"response_format,omitempty"`
}

// h3ImageObject is one image in an H3 response.
//
// An image that failed, or that the content safety check withheld,
// carries only an error.
type h3ImageObject struct {
	URL           string        `json:"url"`
	B64JSON       string        `json:"b64_json"`
	Size          string        `json:"size"`
	RevisedPrompt string        `json:"revised_prompt"`
	Error         *h3ImageError `json:"error"`
}

// h3ImageError is a per-image failure inside an otherwise successful
// response.
type h3ImageError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// h3Usage reports what was actually generated and billed.
type h3Usage struct {
	GeneratedImages int `json:"generated_images"`
	TotalTokens     int `json:"total_tokens"`
	InputTokens     int `json:"input_tokens"`
	OutputTokens    int `json:"output_tokens"`
}

// h3Response is the response body of both H3 endpoints.
type h3Response struct {
	Created      int64           `json:"created"`
	Data         []h3ImageObject `json:"data"`
	OutputFormat string          `json:"output_format"`
	Usage        h3Usage         `json:"usage"`
	RequestID    string          `json:"request_id"`
}

// h3ErrorEnvelope is the OpenAI-style error body H3 uses.
//
// Unlike the Image-01 envelope, which always answers HTTP 200 and puts
// the code in base_resp, this one carries a real HTTP status and folds
// the numeric code into the message, for example "... (1004)".
type h3ErrorEnvelope struct {
	Type  string `json:"type"`
	Error struct {
		Type     string `json:"type"`
		Message  string `json:"message"`
		HTTPCode string `json:"http_code"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

// httpStatus returns the HTTP status carried by the error body.
func (e *h3ErrorEnvelope) httpStatus() int {
	code, err := strconv.Atoi(strings.TrimSpace(e.Error.HTTPCode))
	if err != nil {
		return 0
	}
	return code
}

// statusCode extracts the MiniMax status code from the message.
//
// H3 reports the code as a parenthesised suffix rather than a field, as
// in "login fail: ... (1004)". Returning it lets the rest of Aurore use
// the same codes, and the same user-facing explanations, for both
// families.
func (e *h3ErrorEnvelope) statusCode() int {
	message := strings.TrimSpace(e.Error.Message)
	closeIdx := strings.LastIndex(message, ")")
	if closeIdx < 0 {
		return 0
	}
	openIdx := strings.LastIndex(message[:closeIdx], "(")
	if openIdx < 0 {
		return 0
	}
	code, err := strconv.Atoi(strings.TrimSpace(message[openIdx+1 : closeIdx]))
	if err != nil {
		return 0
	}
	return code
}

// BuildH3Payload converts an ImageRequest into the exact JSON body sent
// to the H3 API, and reports which endpoint to send it to.
//
// The edit endpoint is used whenever a subject reference is present,
// which is how Aurore reaches H3's image editing: the reference image
// travels as a data URI in images[].
//
// H3 has no seed, no style preset and no watermark, so those are
// dropped here rather than at the call sites. Its prompt optimiser is
// expressed inverted, as disable_prompt_enhanced.
func BuildH3Payload(req ImageRequest) (path string, payload map[string]any, err error) {
	if !IsH3Model(req.Model) {
		return "", nil, fmt.Errorf("minimax: model %q is not an H3 model", req.Model)
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return "", nil, fmt.Errorf("minimax: prompt is required")
	}
	prompt = foldNegativePrompt(prompt, strings.TrimSpace(req.NegativePrompt), H3MaxPromptLength)

	payload = map[string]any{
		"model":  string(req.Model),
		"prompt": prompt,
	}

	// Batch size. H3 accepts one more image per request than Image-01.
	n := req.N
	if n <= 0 {
		n = 1
	}
	if n > H3MaxBatchSize {
		n = H3MaxBatchSize
	}
	payload["n"] = n

	// H3 is always asked for inline bytes: the job stores them, and the
	// URL form is not guaranteed to stay available.
	payload["response_format"] = "b64_json"

	// The field is the inverse of the Image-01 prompt_optimizer flag.
	payload["disable_prompt_enhanced"] = !req.PromptOptimizer

	// Output size. An explicit width/height is only sent when it is a
	// valid H3 area, because H3 rejects an out-of-range size instead of
	// scaling it. Otherwise the ratio and area level are used, which is
	// the documented path and cannot fall outside the limits.
	if validH3Size(req.Width, req.Height) {
		payload["size"] = fmt.Sprintf("%dx%d", req.Width, req.Height)
	} else {
		ratio := req.AspectRatio
		if ratio == "" {
			ratio = AspectRatio1x1
		}
		payload["aspect_ratio"] = string(ratio)

		area := req.Area
		if area == "" {
			area = DefaultArea
		}
		payload["area"] = string(area)
	}

	// Reference images select the edit endpoint. The API accepts up to
	// sixteen; Aurore sends the single reference the user attached.
	images := make([]map[string]any, 0, len(req.SubjectReference))
	for _, ref := range req.SubjectReference {
		if ref.ImageFile == "" {
			continue
		}
		images = append(images, map[string]any{"image_url": ref.ImageFile})
		if len(images) >= H3MaxReferenceImages {
			break
		}
	}
	if len(images) > 0 {
		payload["images"] = images
		return H3EditsPath, payload, nil
	}

	return H3GenerationsPath, payload, nil
}

// validH3Size reports whether a width/height pair is one H3 will accept.
//
// The API rejects a size whose area is outside [H3MinArea, H3MaxArea]
// or whose ratio is beyond 1:3..3:1, rather than scaling it, so an
// invalid pair is never sent.
func validH3Size(width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	area := width * height
	if area < H3MinArea || area > H3MaxArea {
		return false
	}
	return width <= height*3 && height <= width*3
}
