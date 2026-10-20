// Package imaging downsizes and JPEG-compresses captures.
package imaging

import (
	"bytes"
	"image"
	"image/jpeg"

	"golang.org/x/image/draw"
)

// OptimizeJPEG scales img down to maxWidth (aspect preserved, never upscaled) and encodes it as JPEG.
func OptimizeJPEG(img image.Image, maxWidth, quality int) ([]byte, error) {
	b := img.Bounds()
	if b.Dx() > maxWidth && maxWidth > 0 {
		h := b.Dy() * maxWidth / b.Dx()
		if h < 1 {
			h = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, maxWidth, h))
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
		img = dst
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
