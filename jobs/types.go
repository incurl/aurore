// Package jobs owns the generation job lifecycle: creation, execution
// against the MiniMax API, persistence and status reporting.
//
// The MiniMax image generation API is synchronous, so a job runs through
// exactly one API call. The job queue exists to serialise requests
// against the documented 10 requests per minute limit, to allow
// cancellation, and to give the UI a durable history.
package jobs

import (
	"github.com/incurl/aurore/minimax"
)

// Params captures the generation settings chosen by the user.
//
// Only fields supported by MiniMax Image-01 are modelled. Unsupported
// diffusion controls (CFG scale, sampler, steps, scheduler, LoRA,
// ControlNet) are never represented.
type Params struct {
	// Model is the MiniMax model to use.
	Model string `json:"model"`
	// AspectRatio is the requested output ratio.
	AspectRatio string `json:"aspect_ratio"`
	// Width and Height are the requested output dimensions.
	Width  int `json:"width"`
	Height int `json:"height"`
	// BatchSize is the number of images to generate (1-9).
	BatchSize int `json:"batch_size"`
	// Seed is the requested seed; 0 means the server picks one.
	Seed int64 `json:"seed"`
	// PromptOptimizer enables the MiniMax prompt optimiser.
	PromptOptimizer bool `json:"prompt_optimizer"`
	// Watermark enables the MiniMax AI watermark.
	Watermark bool `json:"watermark"`
	// StyleType is the MiniMax style preset (image-01-live only).
	StyleType string `json:"style_type"`
	// StyleWeight is the style strength.
	StyleWeight float64 `json:"style_weight"`
	// UseNegativePrompt toggles folding the negative prompt into the
	// positive prompt, because MiniMax has no negative_prompt field.
	UseNegativePrompt bool `json:"use_negative_prompt"`
	// MaskMode selects how the local mask composite is applied.
	MaskMode string `json:"mask_mode"`
}

// Mask composite modes.
const (
	// MaskModeInside keeps the generated pixels inside the mask.
	MaskModeInside = "generated_in_mask"
	// MaskModeOutside keeps the generated pixels outside the mask.
	MaskModeOutside = "generated_outside_mask"
)

// Request is the input for submitting a generation job.
type Request struct {
	// Prompt is the positive prompt.
	Prompt string `json:"prompt"`
	// NegativePrompt is stored on the job and optionally folded into
	// the prompt sent to MiniMax.
	NegativePrompt string `json:"negative_prompt"`
	// Params are the generation settings.
	Params Params `json:"params"`
	// ReferenceAssetID is the uploaded_assets record holding the
	// reference image, if any.
	ReferenceAssetID string `json:"reference_asset_id"`
	// MaskAssetID is the uploaded_assets record holding the mask, if any.
	MaskAssetID string `json:"mask_asset_id"`
}

// Job is the frontend-facing view of a generation job.
type Job struct {
	// ID is the PocketBase record id.
	ID string `json:"id"`
	// Prompt is the positive prompt.
	Prompt string `json:"prompt"`
	// NegativePrompt is the stored negative prompt.
	NegativePrompt string `json:"negative_prompt"`
	// Params are the generation settings.
	Params Params `json:"params"`
	// Status is one of pending/running/completed/failed/cancelled.
	Status string `json:"status"`
	// MiniMaxTaskID is the generation id returned by MiniMax.
	MiniMaxTaskID string `json:"minimax_task_id"`
	// ErrorCode is the MiniMax status code on failure.
	ErrorCode int `json:"error_code"`
	// ErrorMessage is the human readable failure explanation.
	ErrorMessage string `json:"error_message"`
	// ImageIDs lists the generated_images record ids.
	ImageIDs []string `json:"image_ids"`
	// SeedEffective is the seed actually used.
	SeedEffective int64 `json:"seed_effective"`
	// Created and Updated are ISO-8601 timestamps.
	Created string `json:"created"`
	Updated string `json:"updated"`
}

// ExportedImage is a generated image prepared for download.
//
// It carries the format of the bytes, so that the saved file is named
// and typed after the format the image really is, instead of always
// being called ".png".
type ExportedImage struct {
	// Bytes is the image data.
	Bytes []byte `json:"bytes"`
	// Extension is the file extension, without the dot, for example
	// "jpg" or "png".
	Extension string `json:"extension"`
	// Mime is the MIME type, for example "image/jpeg".
	Mime string `json:"mime"`
}

// Image is the frontend-facing view of a generated image.
type Image struct {
	// ID is the PocketBase record id.
	ID string `json:"id"`
	// JobID is the owning job record id.
	JobID string `json:"job_id"`
	// URL is the native PocketBase file URL for the full image.
	URL string `json:"url"`
	// ThumbnailURL is the native PocketBase file URL for the thumbnail.
	ThumbnailURL string `json:"thumbnail_url"`
	// Width and Height are the image dimensions in pixels.
	Width  int `json:"width"`
	Height int `json:"height"`
	// Bytes is the file size.
	Bytes int `json:"bytes"`
	// Mime is the MIME type of the stored image, detected from its
	// bytes when the image was saved.
	Mime string `json:"mime"`
	// Seed is the seed used for this image.
	Seed int64 `json:"seed"`
	// StyleTag is the style preset applied.
	StyleTag string `json:"style_tag"`
	// Prompt is the prompt snapshot.
	Prompt string `json:"prompt"`
	// Created is the ISO-8601 creation timestamp.
	Created string `json:"created"`
}

// toMiniMaxRequest converts a job request into a MiniMax API request.
func (r Request) toMiniMaxRequest(referenceDataURL string) minimax.ImageRequest {
	req := minimax.ImageRequest{
		Model:           minimax.Model(r.Params.Model),
		Prompt:          r.Prompt,
		NegativePrompt:  r.NegativePrompt,
		ResponseFormat:  minimax.ResponseFormatBase64,
		Seed:            r.Params.Seed,
		N:               r.Params.BatchSize,
		PromptOptimizer: r.Params.PromptOptimizer,
		AIGCWatermark:   r.Params.Watermark,
	}

	// The negative prompt is only folded in when the user opted in.
	if !r.Params.UseNegativePrompt {
		req.NegativePrompt = ""
	}

	if r.Params.Model == string(minimax.ModelImage01Live) {
		if r.Params.StyleType != "" {
			req.Style = &minimax.StyleObject{
				StyleType:   minimax.StyleType(r.Params.StyleType),
				StyleWeight: r.Params.StyleWeight,
			}
		}
	} else {
		req.AspectRatio = minimax.AspectRatio(r.Params.AspectRatio)
		req.Width = r.Params.Width
		req.Height = r.Params.Height
	}

	if referenceDataURL != "" {
		req.SubjectReference = []minimax.SubjectReference{{
			Type:      minimax.SubjectReferenceCharacter,
			ImageFile: referenceDataURL,
		}}
	}

	return req
}
