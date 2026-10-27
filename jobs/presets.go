package jobs

import (
	"encoding/json"
	"fmt"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// Preset is a saved prompt template.
type Preset struct {
	// ID is the PocketBase record id.
	ID string `json:"id"`
	// Name is the user friendly preset name.
	Name string `json:"name"`
	// Prompt is the saved positive prompt.
	Prompt string `json:"prompt"`
	// NegativePrompt is the saved negative prompt.
	NegativePrompt string `json:"negative_prompt"`
	// Params are the saved default generation parameters.
	Params Params `json:"params"`
	// IsDefault marks the preset used when the app starts.
	IsDefault bool `json:"is_default"`
	// Created is the ISO-8601 creation timestamp.
	Created string `json:"created"`
}

// ListPresets returns the saved prompt presets.
func (m *Manager) ListPresets() ([]Preset, error) {
	records, err := m.app.FindRecordsByFilter(
		"prompt_presets",
		"",
		"name",
		200,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("list presets: %w", err)
	}

	presets := make([]Preset, 0, len(records))
	for _, record := range records {
		presets = append(presets, toPreset(record))
	}
	return presets, nil
}

// SavePreset creates or updates a prompt preset.
func (m *Manager) SavePreset(preset Preset) (Preset, error) {
	if preset.Name == "" {
		return Preset{}, fmt.Errorf("a preset name is required")
	}

	col, err := m.app.FindCollectionByNameOrId("prompt_presets")
	if err != nil {
		return Preset{}, fmt.Errorf("find prompt_presets collection: %w", err)
	}

	var record *core.Record
	if preset.ID != "" {
		record, err = m.app.FindRecordById("prompt_presets", preset.ID)
		if err != nil {
			return Preset{}, fmt.Errorf("find preset %q: %w", preset.ID, err)
		}
	} else {
		record = core.NewRecord(col)
	}

	record.Set("name", preset.Name)
	record.Set("prompt", preset.Prompt)
	record.Set("negative_prompt", preset.NegativePrompt)
	record.Set("params", preset.Params)
	record.Set("is_default", preset.IsDefault)

	// Only one preset may be the default.
	if preset.IsDefault {
		others, findErr := m.app.FindRecordsByFilter(
			"prompt_presets",
			"id != {:id} && is_default = true",
			"-created",
			200,
			0,
			dbx.Params{"id": record.Id},
		)
		if findErr == nil {
			for _, other := range others {
				other.Set("is_default", false)
				_ = m.app.Save(other)
			}
		}
	}

	if err := m.app.Save(record); err != nil {
		return Preset{}, fmt.Errorf("save preset: %w", err)
	}
	return toPreset(record), nil
}

// DeletePreset removes a prompt preset.
func (m *Manager) DeletePreset(presetID string) error {
	record, err := m.app.FindRecordById("prompt_presets", presetID)
	if err != nil {
		return fmt.Errorf("find preset %q: %w", presetID, err)
	}
	if err := m.app.Delete(record); err != nil {
		return fmt.Errorf("delete preset %q: %w", presetID, err)
	}
	return nil
}

// toPreset converts a PocketBase record into the frontend-facing preset.
func toPreset(record *core.Record) Preset {
	params := Params{}
	if raw := record.Get("params"); raw != nil {
		if encoded, err := json.Marshal(raw); err == nil {
			_ = json.Unmarshal(encoded, &params)
		}
	}

	return Preset{
		ID:             record.Id,
		Name:           record.GetString("name"),
		Prompt:         record.GetString("prompt"),
		NegativePrompt: record.GetString("negative_prompt"),
		Params:         params,
		IsDefault:      record.GetBool("is_default"),
		Created:        record.GetString("created"),
	}
}
