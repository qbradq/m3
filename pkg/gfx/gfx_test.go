package gfx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestMatchRGBToNES(t *testing.T) {
	// Exact matches or close matches
	black := MatchRGBToNES(0, 0, 0)
	if black != 0x0F && black != 0x0D && black != 0x1D && black != 0x2D && black != 0x3D {
		t.Errorf("black matched to %02X", black)
	}

	white := MatchRGBToNES(255, 255, 255)
	if white != 0x30 {
		t.Errorf("white matched to %02X, want 0x30", white)
	}
}

func TestConvertPNGToCHR(t *testing.T) {
	// Create an 8x8 paletted image
	palette := color.Palette{
		color.RGBA{0, 0, 0, 255},       // Color 0: 2BPP %00
		color.RGBA{255, 0, 0, 255},     // Color 1: 2BPP %01
		color.RGBA{0, 255, 0, 255},     // Color 2: 2BPP %10
		color.RGBA{255, 255, 255, 255}, // Color 3: 2BPP %11
	}
	img := image.NewPaletted(image.Rect(0, 0, 8, 8), palette)

	// Row 0: 0, 1, 2, 3, 0, 1, 2, 3 -> bit 0: 0 1 0 1 0 1 0 1 (%01010101 = 0x55)
	//                                   bit 1: 0 0 1 1 0 0 1 1 (%00110011 = 0x33)
	for x := 0; x < 8; x++ {
		img.SetColorIndex(x, 0, uint8(x%4))
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test png: %v", err)
	}

	chrData, err := ConvertPNGToCHR(&buf)
	if err != nil {
		t.Fatalf("ConvertPNGToCHR failed: %v", err)
	}

	if len(chrData) != 16 {
		t.Fatalf("expected 16 bytes for 1 tile, got %d", len(chrData))
	}

	if chrData[0] != 0x55 {
		t.Errorf("plane 0 row 0 = 0x%02X, want 0x55", chrData[0])
	}
	if chrData[8] != 0x33 {
		t.Errorf("plane 1 row 0 = 0x%02X, want 0x33", chrData[8])
	}
}

func TestExtractPNGPalette(t *testing.T) {
	palette := color.Palette{
		color.RGBA{0, 0, 0, 0},         // Transparent -> $0F
		color.RGBA{84, 4, 0, 255},      // Reddish -> $06
		color.RGBA{0, 64, 0, 255},      // Green -> $0A
		color.RGBA{255, 255, 255, 255}, // White -> $30
	}
	img := image.NewPaletted(image.Rect(0, 0, 8, 8), palette)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode test png: %v", err)
	}

	pal, err := ExtractPNGPalette(&buf, 4)
	if err != nil {
		t.Fatalf("ExtractPNGPalette failed: %v", err)
	}

	if len(pal) != 4 {
		t.Fatalf("expected 4 palette entries, got %d", len(pal))
	}

	if pal[0] != 0x0F {
		t.Errorf("pal[0] = 0x%02X, want 0x0F", pal[0])
	}
	if pal[1] != 0x06 {
		t.Errorf("pal[1] = 0x%02X, want 0x06", pal[1])
	}
	if pal[2] != 0x0A && pal[2] != 0x09 {
		t.Errorf("pal[2] = 0x%02X, want 0x0A or 0x09", pal[2])
	}
	if pal[3] != 0x30 {
		t.Errorf("pal[3] = 0x%02X, want 0x30", pal[3])
	}
}
