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

// imageFormat describes a detected image format.
type imageFormat struct {
	// Extension is the file extension, without the dot.
	Extension string
	// Mime is the MIME type.
	Mime string
}

// detectImageFormat identifies the format of the given image bytes.
//
// MiniMax returns JPEG images, but Aurore used to name every stored and
// exported file ".png" and declare it as image/png. The file was then a
// JPEG with a PNG name, which some tools reject and all of them
// misidentify.
//
// The format is read from the bytes rather than from the file name, so
// it stays correct no matter what the file is called.
func detectImageFormat(data []byte) imageFormat {
	switch {
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return imageFormat{Extension: "jpg", Mime: "image/jpeg"}
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return imageFormat{Extension: "png", Mime: "image/png"}
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return imageFormat{Extension: "gif", Mime: "image/gif"}
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return imageFormat{Extension: "webp", Mime: "image/webp"}
	default:
		// An unknown format is still stored, but as a PNG, which is the
		// only one Aurore can always read back.
		return imageFormat{Extension: "png", Mime: "image/png"}
	}
}

// ImageFormat returns the detected file extension and MIME type of the
// given image bytes.
//
// It is exported so that the export path can name the downloaded file
// after the format the bytes really are.
func ImageFormat(data []byte) (extension string, mime string) {
	format := detectImageFormat(data)
	return format.Extension, format.Mime
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
