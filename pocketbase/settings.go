package pocketbase

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// Setting keys used by Aurore.
const (
	// SettingMiniMaxAPIKey holds the encrypted MiniMax API key.
	SettingMiniMaxAPIKey = "minimax.api_key"
	// SettingMiniMaxBaseURL holds the MiniMax API base URL.
	SettingMiniMaxBaseURL = "minimax.base_url"
	// SettingMiniMaxModel holds the selected MiniMax model.
	SettingMiniMaxModel = "minimax.model"
	// SettingDefaults holds the default generation preferences.
	SettingDefaults = "defaults"
	// SettingUI holds the user interface preferences.
	SettingUI = "ui"
)

// UI holds the user interface preferences.
//
// The zoom factor scales the whole interface, because the application
// runs in a webview whose base font size the user cannot change.
type UI struct {
	// Zoom is the interface scale, as a percentage of the base size.
	// It accepts 50 to 200.
	Zoom int `json:"zoom"`
	// Theme is the interface colour scheme: "dark" or "light".
	Theme string `json:"theme"`
}

// DefaultUI returns the factory UI preferences.
func DefaultUI() UI {
	return UI{
		Zoom:  100,
		Theme: "dark",
	}
}

// UI stores the interface preferences.
func (s *SettingsStore) SetUI(ui UI) error {
	return s.setValue(SettingUI, ui, false)
}

// GetUI returns the interface preferences, falling back to the factory
// defaults.
func (s *SettingsStore) GetUI() (UI, error) {
	ui := DefaultUI()
	found, err := s.getValue(SettingUI, false, &ui)
	if err != nil {
		return DefaultUI(), err
	}
	if !found {
		return DefaultUI(), nil
	}
	return ui, nil
}

// DefaultBaseURL is the documented MiniMax API host.
const DefaultBaseURL = "https://api.minimax.cn"

// Defaults holds the default generation preferences.
type Defaults struct {
	// Model is the default MiniMax model.
	Model string `json:"model"`
	// AspectRatio is the default output ratio.
	AspectRatio string `json:"aspect_ratio"`
	// Width and Height are the default output dimensions.
	Width  int `json:"width"`
	Height int `json:"height"`
	// BatchSize is the default number of images per request.
	BatchSize int `json:"batch_size"`
	// Seed is the default seed; 0 means random.
	Seed int64 `json:"seed"`
	// PromptOptimizer enables the MiniMax prompt optimiser by default.
	PromptOptimizer bool `json:"prompt_optimizer"`
	// Watermark enables the MiniMax AI watermark by default.
	Watermark bool `json:"watermark"`
	// StyleType is the default style preset.
	StyleType string `json:"style_type"`
	// StyleWeight is the default style weight.
	StyleWeight float64 `json:"style_weight"`
}

// DefaultDefaults returns the factory defaults.
func DefaultDefaults() Defaults {
	return Defaults{
		Model:           "image-01",
		AspectRatio:     "1:1",
		Width:           1024,
		Height:          1024,
		BatchSize:       1,
		PromptOptimizer: false,
		Watermark:       false,
		StyleType:       "",
		StyleWeight:     0.8,
	}
}

// SettingsStore reads and writes the app_settings collection.
//
// Secrets are stored encrypted and are never exposed to the frontend:
// the app_settings collection has no public list or view rule.
type SettingsStore struct {
	app    core.App
	cipher *Cipher
}

// NewSettingsStore creates a settings store.
func NewSettingsStore(app core.App, cipher *Cipher) *SettingsStore {
	return &SettingsStore{app: app, cipher: cipher}
}

// findRecord looks up the settings record for key. It returns nil when
// the setting has not been written yet.
func (s *SettingsStore) findRecord(key string) (*core.Record, error) {
	record, err := s.app.FindFirstRecordByFilter(
		CollectionAppSettings,
		"key = {:key}",
		map[string]any{"key": key},
	)
	if err != nil {
		// A missing record is not an error: the caller decides whether
		// to create it.
		return nil, nil
	}
	return record, nil
}

// ensureRecord returns the settings record for key, creating it when it
// does not exist yet.
func (s *SettingsStore) ensureRecord(key string) (*core.Record, error) {
	record, err := s.findRecord(key)
	if err != nil {
		return nil, err
	}
	if record != nil {
		return record, nil
	}

	col, err := s.app.FindCollectionByNameOrId(CollectionAppSettings)
	if err != nil {
		return nil, fmt.Errorf("find app_settings collection: %w", err)
	}

	record = core.NewRecord(col)
	record.Set("key", key)
	record.Set("value", json.RawMessage("null"))
	record.Set("encrypted", false)
	if err := s.app.Save(record); err != nil {
		return nil, fmt.Errorf("create setting %q: %w", key, err)
	}
	return record, nil
}

// setValue writes a value for key. When encrypted is true the value is
// encrypted before it is stored.
func (s *SettingsStore) setValue(key string, value any, encrypted bool) error {
	record, err := s.ensureRecord(key)
	if err != nil {
		return err
	}

	if encrypted {
		envelope, encErr := s.cipher.EncryptJSON(value)
		if encErr != nil {
			return encErr
		}
		// The envelope is stored as a JSON string.
		record.Set("value", envelope)
	} else {
		raw, marshalErr := json.Marshal(value)
		if marshalErr != nil {
			return fmt.Errorf("marshal setting %q: %w", key, marshalErr)
		}
		record.Set("value", json.RawMessage(raw))
	}
	record.Set("encrypted", encrypted)

	if err := s.app.Save(record); err != nil {
		return fmt.Errorf("save setting %q: %w", key, err)
	}
	return nil
}

// getValue reads a value for key. When encrypted is true the stored
// envelope is decrypted.
func (s *SettingsStore) getValue(key string, encrypted bool, out any) (bool, error) {
	record, err := s.findRecord(key)
	if err != nil {
		return false, err
	}
	if record == nil {
		return false, nil
	}

	// The value is read as raw JSON so that both JSON documents and
	// JSON strings (the encrypted envelope) round-trip correctly.
	raw, err := rawJSON(record.GetRaw("value"))
	if err != nil {
		return false, fmt.Errorf("read setting %q: %w", key, err)
	}
	if len(raw) == 0 || string(raw) == "null" {
		return false, nil
	}

	if encrypted {
		// The envelope is a JSON string: unwrap it before decrypting.
		var envelope string
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return false, fmt.Errorf("decode setting %q: %w", key, err)
		}
		if err := s.cipher.DecryptJSON(envelope, out); err != nil {
			return false, fmt.Errorf("decrypt setting %q: %w", key, err)
		}
		return true, nil
	}

	if err := json.Unmarshal(raw, out); err != nil {
		return false, fmt.Errorf("unmarshal setting %q: %w", key, err)
	}
	return true, nil
}

// rawJSON normalises a PocketBase JSON field value into raw JSON bytes.
// Depending on how the value was written it may already be a []byte, a
// string or a decoded Go value.
func rawJSON(value any) ([]byte, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []byte:
		return typed, nil
	case string:
		return []byte(typed), nil
	case json.RawMessage:
		return typed, nil
	default:
		return json.Marshal(value)
	}
}

// SetAPIKey stores the MiniMax API key encrypted.
func (s *SettingsStore) SetAPIKey(apiKey string) error {
	return s.setValue(SettingMiniMaxAPIKey, apiKey, true)
}

// GetAPIKey returns the decrypted MiniMax API key. The second return
// value reports whether a key was configured.
func (s *SettingsStore) GetAPIKey() (string, bool, error) {
	var key string
	found, err := s.getValue(SettingMiniMaxAPIKey, true, &key)
	if err != nil {
		return "", false, err
	}
	return key, found, nil
}

// SetBaseURL stores the MiniMax API base URL.
func (s *SettingsStore) SetBaseURL(baseURL string) error {
	return s.setValue(SettingMiniMaxBaseURL, baseURL, false)
}

// GetBaseURL returns the configured MiniMax base URL, falling back to
// the documented default.
func (s *SettingsStore) GetBaseURL() (string, error) {
	var baseURL string
	found, err := s.getValue(SettingMiniMaxBaseURL, false, &baseURL)
	if err != nil {
		return "", err
	}
	if !found || baseURL == "" {
		return DefaultBaseURL, nil
	}
	return baseURL, nil
}

// SetModel stores the selected MiniMax model.
func (s *SettingsStore) SetModel(model string) error {
	return s.setValue(SettingMiniMaxModel, model, false)
}

// GetModel returns the selected MiniMax model, defaulting to image-01.
func (s *SettingsStore) GetModel() (string, error) {
	var model string
	found, err := s.getValue(SettingMiniMaxModel, false, &model)
	if err != nil {
		return "", err
	}
	if !found || model == "" {
		return "image-01", nil
	}
	return model, nil
}

// SetDefaults stores the default generation preferences.
func (s *SettingsStore) SetDefaults(d Defaults) error {
	return s.setValue(SettingDefaults, d, false)
}

// GetDefaults returns the default generation preferences, falling back to
// the factory defaults.
func (s *SettingsStore) GetDefaults() (Defaults, error) {
	d := DefaultDefaults()
	found, err := s.getValue(SettingDefaults, false, &d)
	if err != nil {
		return DefaultDefaults(), err
	}
	if !found {
		return DefaultDefaults(), nil
	}
	return d, nil
}
