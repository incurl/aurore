// Package bindings exposes the Aurore backend to the frontend as Wails
// services.
//
// These are the primary communication channel between the React UI and
// Go. The frontend never talks to PocketBase or MiniMax directly.
package bindings

import (
	"context"
	"fmt"

	"github.com/incurl/aurore/jobs"
	"github.com/incurl/aurore/minimax"
	"github.com/incurl/aurore/pocketbase"
)

// AppService is the main Wails service. It is bound to the frontend and
// exposes every operation the UI needs.
type AppService struct {
	pb       *pocketbase.Service
	settings *pocketbase.SettingsStore
	jobs     *jobs.Manager
}

// NewAppService creates the application service.
func NewAppService(
	pb *pocketbase.Service,
	settings *pocketbase.SettingsStore,
	jobManager *jobs.Manager,
) *AppService {
	return &AppService{pb: pb, settings: settings, jobs: jobManager}
}

// JobManager exposes the job manager so the Wails event bridge can
// subscribe to job updates.
func (s *AppService) JobManager() *jobs.Manager { return s.jobs }

// Settings exposes the settings store.
func (s *AppService) Settings() *pocketbase.SettingsStore { return s.settings }

// Capabilities describes what the selected MiniMax model supports.
//
// The frontend uses this to enable or disable controls. It is the
// single source of truth for the capability gate.
type Capabilities struct {
	// Model is the currently selected model.
	Model string `json:"model"`
	// SupportsDimensions reports whether width/height and aspect_ratio
	// may be set. Only image-01 supports them.
	SupportsDimensions bool `json:"supports_dimensions"`
	// SupportsStyle reports whether a style preset may be selected.
	// Only image-01-live supports it.
	SupportsStyle bool `json:"supports_style"`
	// SupportsReferenceImage reports whether a reference image may be
	// attached.
	SupportsReferenceImage bool `json:"supports_reference_image"`
	// SupportsNegativePrompt reports whether the negative prompt can be
	// sent as a separate field. MiniMax has no negative_prompt field,
	// so this is always false; the UI instead offers to append the
	// negative prompt to the positive prompt.
	SupportsNegativePrompt bool `json:"supports_negative_prompt"`
	// SupportsMask reports whether the mask can be sent to the model.
	// MiniMax has no mask field, so this is always false; the UI
	// instead offers a local mask composite.
	SupportsMask bool `json:"supports_mask"`
	// AspectRatios lists the selectable aspect ratios.
	AspectRatios []string `json:"aspect_ratios"`
	// StyleTypes lists the selectable style presets.
	StyleTypes []string `json:"style_types"`
	// MinDimension and MaxDimension bound the dimension inputs.
	MinDimension int `json:"min_dimension"`
	// MaxDimension bounds the dimension inputs.
	MaxDimension int `json:"max_dimension"`
	// MaxBatchSize is the largest batch MiniMax accepts.
	MaxBatchSize int `json:"max_batch_size"`
	// MaxPromptLength is the prompt character limit.
	MaxPromptLength int `json:"max_prompt_length"`
}

// GetCapabilities returns the capability description for a model. When
// model is empty the currently configured model is used.
func (s *AppService) GetCapabilities(model string) (Capabilities, error) {
	if model == "" {
		configured, err := s.settings.GetModel()
		if err != nil {
			return Capabilities{}, err
		}
		model = configured
	}

	capabilities := Capabilities{
		Model:                  model,
		SupportsDimensions:     minimax.SupportsDimensions(minimax.Model(model)),
		SupportsStyle:          minimax.SupportsStyle(minimax.Model(model)),
		SupportsReferenceImage: true,
		// MiniMax has neither a negative_prompt nor a mask field.
		SupportsNegativePrompt: false,
		SupportsMask:           false,
		MinDimension:           minimax.MinDimension,
		MaxDimension:           minimax.MaxDimension,
		MaxBatchSize:           minimax.MaxBatchSize,
		MaxPromptLength:        minimax.MaxPromptLength,
	}

	for _, ratio := range minimax.AllAspectRatios() {
		capabilities.AspectRatios = append(capabilities.AspectRatios, string(ratio))
	}
	for _, style := range minimax.AllStyleTypes() {
		capabilities.StyleTypes = append(capabilities.StyleTypes, string(style))
	}

	return capabilities, nil
}

// SettingsView is the frontend-facing view of the application settings.
//
// The API key is never included: it is write-only from the UI's point of
// view.
type SettingsView struct {
	// HasAPIKey reports whether an API key is configured.
	HasAPIKey bool `json:"has_api_key"`
	// BaseURL is the MiniMax API host.
	BaseURL string `json:"base_url"`
	// Model is the selected MiniMax model.
	Model string `json:"model"`
	// Defaults are the default generation preferences.
	Defaults pocketbase.Defaults `json:"defaults"`
}

// GetSettings returns the current settings.
func (s *AppService) GetSettings() (SettingsView, error) {
	view := SettingsView{}

	_, hasKey, err := s.settings.GetAPIKey()
	if err != nil {
		return SettingsView{}, err
	}
	view.HasAPIKey = hasKey

	baseURL, err := s.settings.GetBaseURL()
	if err != nil {
		return SettingsView{}, err
	}
	view.BaseURL = baseURL

	model, err := s.settings.GetModel()
	if err != nil {
		return SettingsView{}, err
	}
	view.Model = model

	defaults, err := s.settings.GetDefaults()
	if err != nil {
		return SettingsView{}, err
	}
	view.Defaults = defaults

	return view, nil
}

// SaveCredentials stores the MiniMax credentials.
//
// An empty apiKey leaves the stored key untouched, so the UI can save
// other settings without re-entering the secret.
func (s *AppService) SaveCredentials(apiKey string, baseURL string, model string) error {
	if apiKey != "" {
		if err := s.settings.SetAPIKey(apiKey); err != nil {
			return err
		}
	}
	if baseURL != "" {
		if err := s.settings.SetBaseURL(baseURL); err != nil {
			return err
		}
	}
	if model != "" {
		if err := s.settings.SetModel(model); err != nil {
			return err
		}
	}
	return nil
}

// SaveDefaults stores the default generation preferences.
func (s *AppService) SaveDefaults(defaults pocketbase.Defaults) error {
	return s.settings.SetDefaults(defaults)
}

// TestCredentials verifies the stored MiniMax credentials.
//
// It does not generate an image, so it consumes no quota.
func (s *AppService) TestCredentials() error {
	apiKey, found, err := s.settings.GetAPIKey()
	if err != nil {
		return err
	}
	if !found || apiKey == "" {
		return fmt.Errorf("no MiniMax API key is configured")
	}

	baseURL, err := s.settings.GetBaseURL()
	if err != nil {
		return err
	}

	client := minimax.NewClient(apiKey, minimax.WithBaseURL(baseURL))
	return client.Ping(context.Background())
}

// SubmitJob queues a generation job.
func (s *AppService) SubmitJob(request jobs.Request) (jobs.Job, error) {
	return s.jobs.Submit(context.Background(), request)
}

// CancelJob cancels a pending or running job.
func (s *AppService) CancelJob(jobID string) error {
	return s.jobs.Cancel(jobID)
}

// RetryJob re-runs a job with the same parameters.
func (s *AppService) RetryJob(jobID string) (jobs.Job, error) {
	job, err := s.jobs.GetJob(jobID)
	if err != nil {
		return jobs.Job{}, err
	}

	request := jobs.Request{
		Prompt:         job.Prompt,
		NegativePrompt: job.NegativePrompt,
		Params:         job.Params,
	}
	return s.jobs.Submit(context.Background(), request)
}

// ListJobs returns the recent jobs.
func (s *AppService) ListJobs(limit int) ([]jobs.Job, error) {
	return s.jobs.ListJobs(limit)
}

// GetJob returns a single job.
func (s *AppService) GetJob(jobID string) (jobs.Job, error) {
	return s.jobs.GetJob(jobID)
}

// ListImages returns the generated images.
func (s *AppService) ListImages(jobID string, limit int) ([]jobs.Image, error) {
	return s.jobs.ListImages(jobID, limit)
}

// DeleteImage removes a generated image.
func (s *AppService) DeleteImage(imageID string) error {
	return s.jobs.DeleteImage(imageID)
}

// UploadAsset stores an uploaded reference image or mask.
func (s *AppService) UploadAsset(kind string, data []byte, filename string) (jobs.Asset, error) {
	return s.jobs.UploadAsset(kind, data, filename)
}

// ListAssets returns the uploaded assets.
func (s *AppService) ListAssets(kind string, limit int) ([]jobs.Asset, error) {
	return s.jobs.ListAssets(kind, limit)
}

// DeleteAsset removes an uploaded asset.
func (s *AppService) DeleteAsset(assetID string) error {
	return s.jobs.DeleteAsset(assetID)
}

// ListPresets returns the saved prompt presets.
func (s *AppService) ListPresets() ([]jobs.Preset, error) {
	return s.jobs.ListPresets()
}

// SavePreset creates or updates a prompt preset.
func (s *AppService) SavePreset(preset jobs.Preset) (jobs.Preset, error) {
	return s.jobs.SavePreset(preset)
}

// DeletePreset removes a prompt preset.
func (s *AppService) DeletePreset(presetID string) error {
	return s.jobs.DeletePreset(presetID)
}

// ExportImage returns the bytes of a generated image so the frontend can
// save it to disk.
func (s *AppService) ExportImage(imageID string) ([]byte, error) {
	return s.jobs.ExportImageBytes(imageID)
}
