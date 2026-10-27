package jobs

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
)

// imageDimensions decodes the image header and returns its pixel
// dimensions. It returns zeroes when the data cannot be decoded.
func imageDimensions(data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// ImageSize describes the pixel dimensions of an image.
type ImageSize struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// DimensionsOf returns the pixel dimensions of the given image bytes.
func DimensionsOf(data []byte) (ImageSize, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return ImageSize{}, fmt.Errorf("decode image: %w", err)
	}
	return ImageSize{Width: cfg.Width, Height: cfg.Height}, nil
}

// encodePNG writes the image to w in PNG format.
func encodePNG(w io.Writer, img image.Image) error {
	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}
	return nil
}
