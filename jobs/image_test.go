package jobs

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// jpegBytes encodes a small JPEG.
func jpegBytes(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// pngBytesOf encodes a small PNG.
func pngBytesOf(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{G: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// TestDetectImageFormat verifies that the format is read from the bytes.
//
// MiniMax returns JPEG images. Aurore used to name every stored and
// exported file ".png" and declare it as image/png, so the file was a
// JPEG with a PNG name, which tools misidentify.
func TestDetectImageFormat(t *testing.T) {
	cases := []struct {
		name      string
		data      []byte
		extension string
		mime      string
	}{
		{"jpeg", jpegBytes(t), "jpg", "image/jpeg"},
		{"png", pngBytesOf(t), "png", "image/png"},
		{"gif", []byte("GIF89a...."), "gif", "image/gif"},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "webp", "image/webp"},
		{"unknown falls back to png", []byte("not an image"), "png", "image/png"},
		{"empty falls back to png", nil, "png", "image/png"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extension, mime := ImageFormat(tc.data)
			if extension != tc.extension {
				t.Errorf("extension = %q, want %q", extension, tc.extension)
			}
			if mime != tc.mime {
				t.Errorf("mime = %q, want %q", mime, tc.mime)
			}
		})
	}
}

// TestDetectImageFormatHandlesTruncatedData verifies that a short input
// is not mistaken for a format it only starts with.
func TestDetectImageFormatHandlesTruncatedData(t *testing.T) {
	cases := [][]byte{
		{0xFF},
		{0xFF, 0xD8},
		{0x89, 'P', 'N', 'G'},
		[]byte("GIF"),
		[]byte("RIFF"),
	}

	for _, data := range cases {
		extension, mime := ImageFormat(data)
		if extension != "png" || mime != "image/png" {
			t.Errorf("ImageFormat(%v) = %q/%q, want png/image/png", data, extension, mime)
		}
	}
}
