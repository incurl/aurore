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

// Limits for the H3 family. They are larger than the Image-01 ones on
// the limits that changed, and H3 has no width/height pair at all.
const (
	// H3MaxPromptLength is the maximum prompt length for H3.
	H3MaxPromptLength = 32000

	// H3MaxBatchSize is the largest "n" H3 accepts.
	H3MaxBatchSize = 10

	// H3MaxReferenceImages is the largest number of reference images the
	// H3 edit endpoint accepts in one request.
	H3MaxReferenceImages = 16

	// H3MinArea and H3MaxArea bound the pixel area of an H3 output. They
	// matter because an explicit size below the minimum is rejected
	// outright rather than scaled, so Aurore never sends one.
	H3MinArea = 655360
	H3MaxArea = 20000000
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
	prompt = foldNegativePrompt(prompt, negative, MaxPromptLength)

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

// foldNegativePrompt appends a negative prompt to the positive one.
//
// Neither MiniMax family has a negative_prompt field, so the constraint
// can only reach the model as part of the prompt. The result is
// truncated to limit characters.
func foldNegativePrompt(prompt, negative string, limit int) string {
	if negative != "" {
		if len(negative) > MaxNegativePromptLength {
			negative = negative[:MaxNegativePromptLength]
		}
		prompt = prompt + " " + NegativePromptPrefix + negative + "."
	}
	if len(prompt) > limit {
		prompt = prompt[:limit]
	}
	return prompt
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
// SupportsDimensions reports whether the model accepts explicit
// width/height parameters.
//
// H3 does not: it takes an aspect ratio plus an area level, and the
// server derives the dimensions. Only image-01 accepts a raw size.
func SupportsDimensions(model Model) bool {
	return model == ModelImage01
}

// SupportsAspectRatio reports whether the model accepts an aspect ratio.
//
// image-01 and image-01-live both do, and so does H3; only H3 takes it
// alongside an area level instead of a width and height.
func SupportsAspectRatio(model Model) bool {
	return model != ModelImage01Live
}

// SupportsStyle reports whether the model accepts a style preset.
func SupportsStyle(model Model) bool {
	return model == ModelImage01Live
}

// SupportsSeed reports whether the model accepts a seed.
//
// H3 has no seed field at all, so a seed the user sets cannot reach it
// and must not be presented as if it could.
func SupportsSeed(model Model) bool {
	return !IsH3Model(model)
}

// SupportsWatermark reports whether the model accepts the AIGC
// watermark flag. H3 has no such field.
func SupportsWatermark(model Model) bool {
	return !IsH3Model(model)
}

// SupportsImageEdit reports whether the model can take reference
// images.
//
// H3 reaches them through its edit endpoint rather than a
// subject_reference field, but the effect is the same.
func SupportsImageEdit(model Model) bool {
	return true
}
