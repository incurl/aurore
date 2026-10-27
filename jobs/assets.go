package jobs

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// Asset is the frontend-facing view of an uploaded asset.
type Asset struct {
	// ID is the PocketBase record id.
	ID string `json:"id"`
	// Kind is "reference" or "mask".
	Kind string `json:"kind"`
	// JobID is the owning job, if the asset was used.
	JobID string `json:"job_id"`
	// URL is the native PocketBase file URL.
	URL string `json:"url"`
	// Mime is the stored MIME type.
	Mime string `json:"mime"`
	// Bytes is the file size.
	Bytes int `json:"bytes"`
	// Width and Height are the pixel dimensions.
	Width  int `json:"width"`
	Height int `json:"height"`
	// Created is the ISO-8601 creation timestamp.
	Created string `json:"created"`
}

// UploadAsset stores a user-uploaded reference image or mask.
//
// The file is stored using PocketBase's native file field, so it is
// served by the built-in file route and inherits the collection's access
// rules.
func (m *Manager) UploadAsset(kind string, data []byte, filename string) (Asset, error) {
	if kind != "reference" && kind != "mask" {
		return Asset{}, fmt.Errorf("unknown asset kind %q", kind)
	}
	if len(data) == 0 {
		return Asset{}, fmt.Errorf("the uploaded file is empty")
	}

	col, err := m.app.FindCollectionByNameOrId("uploaded_assets")
	if err != nil {
		return Asset{}, fmt.Errorf("find uploaded_assets collection: %w", err)
	}

	if filename == "" {
		filename = "upload.png"
	}

	file, err := filesystem.NewFileFromBytes(data, filename)
	if err != nil {
		return Asset{}, fmt.Errorf("create file: %w", err)
	}

	record := core.NewRecord(col)
	record.Set("kind", kind)
	record.Set("file", file)
	record.Set("mime", detectMimeType(filename))
	record.Set("bytes", len(data))

	if size, dimErr := DimensionsOf(data); dimErr == nil {
		record.Set("width", size.Width)
		record.Set("height", size.Height)
	}

	if err := m.app.Save(record); err != nil {
		return Asset{}, fmt.Errorf("save asset: %w", err)
	}

	return m.toAsset(record), nil
}

// ListAssets returns the uploaded assets, newest first.
func (m *Manager) ListAssets(kind string, limit int) ([]Asset, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	filter := ""
	var params []dbx.Params
	if kind != "" {
		filter = "kind = {:kind}"
		params = append(params, dbx.Params{"kind": kind})
	}

	records, err := m.app.FindRecordsByFilter(
		"uploaded_assets",
		filter,
		"-created",
		limit,
		0,
		params...,
	)
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}

	assets := make([]Asset, 0, len(records))
	for _, record := range records {
		assets = append(assets, m.toAsset(record))
	}
	return assets, nil
}

// DeleteAsset removes an uploaded asset.
func (m *Manager) DeleteAsset(assetID string) error {
	record, err := m.app.FindRecordById("uploaded_assets", assetID)
	if err != nil {
		return fmt.Errorf("find asset %q: %w", assetID, err)
	}
	if err := m.app.Delete(record); err != nil {
		return fmt.Errorf("delete asset %q: %w", assetID, err)
	}
	return nil
}

// AssetBytes returns the raw bytes of an uploaded asset.
func (m *Manager) AssetBytes(assetID string) ([]byte, error) {
	record, err := m.app.FindRecordById("uploaded_assets", assetID)
	if err != nil {
		return nil, fmt.Errorf("find asset %q: %w", assetID, err)
	}
	return readFileBytes(record)
}

// readFileBytes reads the bytes of a record's file attachment.
func readFileBytes(record *core.Record) ([]byte, error) {
	file, ok := record.Get("file").(*filesystem.File)
	if !ok || file == nil {
		return nil, fmt.Errorf("record has no file")
	}
	reader, err := file.Reader.Open()
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

// detectMimeType infers a MIME type from a file name.
func detectMimeType(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

// toAsset converts a PocketBase record into the frontend-facing asset.
func (m *Manager) toAsset(record *core.Record) Asset {
	asset := Asset{
		ID:      record.Id,
		Kind:    record.GetString("kind"),
		JobID:   record.GetString("job"),
		Mime:    record.GetString("mime"),
		Bytes:   record.GetInt("bytes"),
		Width:   record.GetInt("width"),
		Height:  record.GetInt("height"),
		Created: record.GetString("created"),
	}

	if name := record.GetString("file"); name != "" {
		asset.URL = "/api/files/uploaded_assets/" + record.Id + "/" + name
	}
	return asset
}
