package imaging

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

func TestOptimizeJPEG(t *testing.T) {
	tests := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"downscale keeps aspect", 2560, 1440, 1280, 720},
		{"never upscales", 800, 600, 800, 600},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := OptimizeJPEG(image.NewRGBA(image.Rect(0, 0, tt.w, tt.h)), 1280, 80)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("output is not a JPEG: %v", err)
			}
			if cfg.Width != tt.wantW || cfg.Height != tt.wantH {
				t.Errorf("size = %dx%d, want %dx%d", cfg.Width, cfg.Height, tt.wantW, tt.wantH)
			}
		})
	}
}
