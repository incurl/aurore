package minimax

import (
	"fmt"
	"strings"
)

// Limits enforced by the MiniMax image generation API.
const (
	// MaxPromptLength is the maximum prompt length in characters.
	MaxPromptLength = 1500

	// MinDimension and MaxDimension bound the width/height parameters.
	MinDimension = 512
	MaxDimension = 2048

	// DimensionMultiple is the required multiple of width and height.
	DimensionMultiple = 8

	// MinBatchSize and MaxBatchSize bound the "n" parameter.
	MinBatchSize = 1
	MaxBatchSize = 9

	// MaxReferenceImageBytes is the documented size cap for a reference
	// image (10 MB).
	MaxReferenceImageBytes = 10 << 20

	// MaxNegativePromptLength bounds the locally folded negative prompt.
	MaxNegativePromptLength = 500
)

// NegativePromptPrefix is prepended to the negative prompt when it is
// folded into the positive prompt. MiniMax has no negative_prompt field,
// so Aurore appends the constraint to the single supported prompt field.
const NegativePromptPrefix = "Avoid: "

// BuildPayload converts an ImageRequest into the exact JSON body sent to
// MiniMax. It is the single authoritative place where parameter mapping
// and model-specific gating happen.
//
// Rules applied, in order:
//
//   - The prompt is truncated to MaxPromptLength characters.
//   - If a negative prompt is present it is folded into the prompt as
//     "Avoid: <text>." because the API has no negative_prompt field.
//   - The model is validated; an empty model defaults to ModelImage01.
//   - aspect_ratio / width / height are dropped for ModelImage01Live,
//     which does not support them.
//   - style is dropped for ModelImage01, which does not support it.
//   - width/height must be a pair, in range and a multiple of 8; an
//     invalid pair is dropped in favour of aspect_ratio.
//   - n is clamped to [1, 9]; 0 means 1.
//   - seed is passed through; 0 means the server picks a random seed.
//   - At most one subject_reference is sent, as the API supports a single
//     reference image.
//
// The returned map is the literal request body.
func BuildPayload(req ImageRequest) (map[string]any, error) {
	if req.Model == "" {
		req.Model = ModelImage01
	}
	if req.Model != ModelImage01 && req.Model != ModelImage01Live {
		return nil, fmt.Errorf("minimax: unsupported model %q", req.Model)
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return nil, fmt.Errorf("minimax: prompt is required")
	}

	negative := strings.TrimSpace(req.NegativePrompt)
	if negative != "" {
		if len(negative) > MaxNegativePromptLength {
			negative = negative[:MaxNegativePromptLength]
		}
		prompt = prompt + " " + NegativePromptPrefix + negative + "."
	}
	if len(prompt) > MaxPromptLength {
		prompt = prompt[:MaxPromptLength]
	}

	payload := map[string]any{
		"model":  string(req.Model),
		"prompt": prompt,
	}

	// Batch size.
	n := req.N
	if n == 0 {
		n = 1
	}
	if n < MinBatchSize {
		n = MinBatchSize
	}
	if n > MaxBatchSize {
		n = MaxBatchSize
	}
	payload["n"] = n

	// Seed: 0 means "server picks a random seed", so it is omitted.
	if req.Seed != 0 {
		payload["seed"] = req.Seed
	}

	if req.PromptOptimizer {
		payload["prompt_optimizer"] = true
	}
	if req.AIGCWatermark {
		payload["aigc_watermark"] = true
	}

	// Response format. Aurore always requests base64 so that the image
	// bytes can be persisted immediately; the URL form expires after 24h.
	switch req.ResponseFormat {
	case ResponseFormatBase64, ResponseFormatURL:
		payload["response_format"] = string(req.ResponseFormat)
	default:
		payload["response_format"] = string(ResponseFormatBase64)
	}

	// Style presets apply only to image-01-live.
	if req.Model == ModelImage01Live && req.Style != nil && req.Style.StyleType != "" {
		style := map[string]any{"style_type": string(req.Style.StyleType)}
		if req.Style.StyleWeight > 0 && req.Style.StyleWeight <= 1 {
			style["style_weight"] = req.Style.StyleWeight
		}
		payload["style"] = style
	}

	// Dimensions apply only to image-01.
	if req.Model == ModelImage01 {
		width, height := req.Width, req.Height
		if validDimensions(width, height) {
			payload["width"] = width
			payload["height"] = height
		} else if req.AspectRatio != "" {
			payload["aspect_ratio"] = string(req.AspectRatio)
		}
	}

	// Reference image: the API supports a single subject reference.
	if len(req.SubjectReference) > 0 {
		ref := req.SubjectReference[0]
		if ref.ImageFile != "" {
			subjectType := ref.Type
			if subjectType == "" {
				subjectType = SubjectReferenceCharacter
			}
			payload["subject_reference"] = []map[string]any{{
				"type":       string(subjectType),
				"image_file": ref.ImageFile,
			}}
		}
	}

	return payload, nil
}

// validDimensions reports whether a width/height pair satisfies the
// documented constraints: both set, within [512, 2048] and each a
// multiple of 8.
func validDimensions(width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	if width < MinDimension || width > MaxDimension {
		return false
	}
	if height < MinDimension || height > MaxDimension {
		return false
	}
	if width%DimensionMultiple != 0 || height%DimensionMultiple != 0 {
		return false
	}
	return true
}

// SupportsDimensions reports whether the model accepts explicit
// width/height or aspect_ratio parameters.
func SupportsDimensions(model Model) bool {
	return model != ModelImage01Live
}

// SupportsStyle reports whether the model accepts a style preset.
func SupportsStyle(model Model) bool {
	return model == ModelImage01Live
}
