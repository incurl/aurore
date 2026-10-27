package jobs

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// MaskComposite applies a mask to a generated image.
//
// MiniMax Image-01 has no alpha mask input, so Aurore performs the
// composite locally: the mask is drawn over the generated image and the
// result is the combination of the two according to the mask mode.
//
//   - MaskModeInside keeps the generated pixels where the mask is opaque
//     and the original image elsewhere.
//   - MaskModeOutside keeps the generated pixels where the mask is
//     transparent and the original image inside the mask.
//
// This is explicitly a local composite, not model-side inpainting. The
// UI labels it as such.
func MaskComposite(generated, original, mask []byte, mode string) ([]byte, error) {
	if len(mask) == 0 {
		return generated, nil
	}

	genImg, _, err := image.Decode(bytes.NewReader(generated))
	if err != nil {
		return nil, fmt.Errorf("decode generated image: %w", err)
	}
	maskImg, _, err := image.Decode(bytes.NewReader(mask))
	if err != nil {
		return nil, fmt.Errorf("decode mask: %w", err)
	}

	// The original is optional: without it, the areas outside the mask
	// are left transparent rather than filled.
	var origImg image.Image
	if len(original) > 0 {
		origImg, _, err = image.Decode(bytes.NewReader(original))
		if err != nil {
			return nil, fmt.Errorf("decode original image: %w", err)
		}
	}

	bounds := genImg.Bounds()
	out := image.NewNRGBA(bounds)

	// Start from the original (or transparent) so the untouched areas
	// keep the source content.
	if origImg != nil {
		draw.Draw(out, bounds, origImg, origImg.Bounds().Min, draw.Src)
	}

	// Normalise the mask onto the generated image bounds.
	maskNorm := resizeNearest(maskImg, bounds)

	switch mode {
	case MaskModeOutside:
		// Keep the generated pixels where the mask is transparent.
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				_, _, _, a := maskNorm.At(x, y).RGBA()
				if a == 0 {
					draw.Draw(out, image.Rect(x, y, x+1, y+1), genImg, image.Point{x, y}, draw.Src)
				}
			}
		}
	default:
		// MaskModeInside: keep the generated pixels where the mask is
		// opaque.
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				_, _, _, a := maskNorm.At(x, y).RGBA()
				if a > 0 {
					draw.Draw(out, image.Rect(x, y, x+1, y+1), genImg, image.Point{x, y}, draw.Src)
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := encodePNG(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// resizeNearest scales src to fit dst using nearest-neighbour sampling.
// It preserves the hard edges of a drawn mask.
func resizeNearest(src image.Image, dst image.Rectangle) *image.NRGBA {
	out := image.NewNRGBA(dst)
	srcBounds := src.Bounds()
	if srcBounds.Empty() {
		return out
	}

	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	dstW := dst.Dx()
	dstH := dst.Dy()

	for y := 0; y < dstH; y++ {
		sy := srcBounds.Min.Y + y*srcH/dstH
		for x := 0; x < dstW; x++ {
			sx := srcBounds.Min.X + x*srcW/dstW
			out.Set(dst.Min.X+x, dst.Min.Y+y, src.At(sx, sy))
		}
	}
	return out
}

// Thumbnail renders a small preview of the image bytes.
func Thumbnail(data []byte, maxSize int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return nil, fmt.Errorf("image has no dimensions")
	}

	// Only shrink; never upscale.
	if w <= maxSize && h <= maxSize {
		var buf bytes.Buffer
		if err := encodePNG(&buf, img); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}

	scale := maxSize
	if w > h {
		scale = maxSize * w / h
	} else {
		scale = maxSize * h / w
	}
	// Guard against an inverted aspect ratio.
	if scale < maxSize {
		scale = maxSize
	}

	dst := image.Rect(0, 0, maxSize, maxSize)
	small := resizeNearest(img, dst)

	var buf bytes.Buffer
	if err := encodePNG(&buf, small); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
