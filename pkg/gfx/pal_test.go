package gfx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestParsePalValid(t *testing.T) {
	input := `
0:
$0F
$00
$10
$30

1:
$0F
$01
$21
$0F
`
	pal, err := ParsePal(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParsePal failed: %v", err)
	}

	if len(pal.Palettes) != 2 {
		t.Fatalf("expected 2 sub-palettes, got %d", len(pal.Palettes))
	}

	if pal.Palettes[0].Slot != 0 || len(pal.Palettes[0].Colors) != 4 {
		t.Errorf("unexpected slot 0: %+v", pal.Palettes[0])
	}
	expected0 := []byte{0x0F, 0x00, 0x10, 0x30}
	for i, c := range expected0 {
		if pal.Palettes[0].Colors[i] != c {
			t.Errorf("slot 0 color %d = %02X, want %02X", i, pal.Palettes[0].Colors[i], c)
		}
	}

	if pal.Palettes[1].Slot != 1 || len(pal.Palettes[1].Colors) != 4 {
		t.Errorf("unexpected slot 1: %+v", pal.Palettes[1])
	}
	expected1 := []byte{0x0F, 0x01, 0x21, 0x0F}
	for i, c := range expected1 {
		if pal.Palettes[1].Colors[i] != c {
			t.Errorf("slot 1 color %d = %02X, want %02X", i, pal.Palettes[1].Colors[i], c)
		}
	}

	bytes := pal.ToBytes()
	if len(bytes) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(bytes))
	}
}

func TestParsePalErrors(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		errContains string
	}{
		{
			name: "Color 0D error",
			input: `0:
$0F
$0D
$00
$30`,
			errContains: "forbidden color $0D",
		},
		{
			name: "Color 20 error",
			input: `0:
$0F
$00
$20
$30`,
			errContains: "forbidden color $20",
		},
		{
			name: "More than 4 colors",
			input: `0:
$0F
$00
$10
$30
$01`,
			errContains: "contains more than 4 colors",
		},
		{
			name: "Out of numeric order - decreasing",
			input: `1:
$0F
$10
0:
$0F
$00`,
			errContains: "out of numeric order",
		},
		{
			name: "Out of numeric order - duplicate slot",
			input: `0:
$0F
0:
$10`,
			errContains: "out of numeric order",
		},
		{
			name: "Color before slot header",
			input: `$0F
0:
$10`,
			errContains: "color defined before palette slot header",
		},
		{
			name: "Invalid line format",
			input: `0:
hello`,
			errContains: "invalid palette line",
		},
		{
			name: "Color out of NES range",
			input: `0:
$45`,
			errContains: "out of NES palette range",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParsePal(strings.NewReader(tc.input))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("expected error to contain %q, got %v", tc.errContains, err)
			}
		})
	}
}

func TestConvertPNGToCHRWithPalette(t *testing.T) {
	palInput := `
0:
$0F
$06
$0A
$30
`
	pal, err := ParsePal(strings.NewReader(palInput))
	if err != nil {
		t.Fatalf("ParsePal failed: %v", err)
	}

	// 8x8 image using distinct NES colors:
	// NES $0F: (0,0,0) -> %00 (color 0)
	// NES $06: (84, 4, 0) -> %01 (color 1)
	// NES $0A: (0, 64, 0) -> %10 (color 2)
	// NES $30: (255, 255, 255) -> %11 (color 3)
	c0 := color.RGBA{NESPaletteRGB[0x0F][0], NESPaletteRGB[0x0F][1], NESPaletteRGB[0x0F][2], 255}
	c1 := color.RGBA{NESPaletteRGB[0x06][0], NESPaletteRGB[0x06][1], NESPaletteRGB[0x06][2], 255}
	c2 := color.RGBA{NESPaletteRGB[0x0A][0], NESPaletteRGB[0x0A][1], NESPaletteRGB[0x0A][2], 255}
	c3 := color.RGBA{NESPaletteRGB[0x30][0], NESPaletteRGB[0x30][1], NESPaletteRGB[0x30][2], 255}

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	// Row 0: c0, c1, c2, c3, c0, c1, c2, c3
	for x := 0; x < 8; x++ {
		switch x % 4 {
		case 0:
			img.Set(x, 0, c0)
		case 1:
			img.Set(x, 0, c1)
		case 2:
			img.Set(x, 0, c2)
		case 3:
			img.Set(x, 0, c3)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}

	chrData, err := ConvertPNGToCHRWithPalette(&buf, pal)
	if err != nil {
		t.Fatalf("ConvertPNGToCHRWithPalette failed: %v", err)
	}

	if len(chrData) != 16 {
		t.Fatalf("expected 16 bytes for 1 tile, got %d", len(chrData))
	}

	// Row 0: 0, 1, 2, 3, 0, 1, 2, 3 -> plane 0: %01010101 (0x55), plane 1: %00110011 (0x33)
	if chrData[0] != 0x55 {
		t.Errorf("plane 0 row 0 = 0x%02X, want 0x55", chrData[0])
	}
	if chrData[8] != 0x33 {
		t.Errorf("plane 1 row 0 = 0x%02X, want 0x33", chrData[8])
	}
}

func TestConvertPNGToCHRWithPaletteMismatchError(t *testing.T) {
	palInput := `
0:
$0F
$00
$10
$30
`
	pal, err := ParsePal(strings.NewReader(palInput))
	if err != nil {
		t.Fatalf("ParsePal failed: %v", err)
	}

	// 8x8 image using a color NOT in the palette (e.g. Red $06)
	cRed := color.RGBA{NESPaletteRGB[0x06][0], NESPaletteRGB[0x06][1], NESPaletteRGB[0x06][2], 255}
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, cRed)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}

	_, err = ConvertPNGToCHRWithPalette(&buf, pal)
	if err == nil {
		t.Fatalf("expected error when tile contains color not in palette, got nil")
	}
	if !strings.Contains(err.Error(), "$06") {
		t.Errorf("expected error message to include offending color index $06, got: %v", err)
	}
}

func TestRealAssetsConversion(t *testing.T) {
	inspect := func(name, palPath, pngPath string) {
		f, err := os.Open(pngPath)
		if err != nil {
			t.Fatalf("open %s failed: %v", pngPath, err)
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			t.Fatalf("decode %s failed: %v", pngPath, err)
		}
		t.Logf("=== %s ===", name)
		if palImg, ok := img.(*image.Paletted); ok {
			t.Logf("Is Paletted PNG! Palette len: %d", len(palImg.Palette))
			for i, c := range palImg.Palette {
				r, g, b, a := c.RGBA()
				t.Logf("  PNG pal[%d]: RGBA=(%d, %d, %d, %d), MatchNES=$%02X", i, r>>8, g>>8, b>>8, a>>8, MatchRGBToNES(uint8(r>>8), uint8(g>>8), uint8(b>>8)))
			}
		}
		uniqueRGB := make(map[[3]uint8]int)
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				c := img.At(x, y)
				r, g, b, a := c.RGBA()
				if a >= 32768 {
					uniqueRGB[[3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}]++
				}
			}
		}
		for rgb, count := range uniqueRGB {
			t.Logf("  Unique RGB: (%d, %d, %d) count=%d -> MatchNES=$%02X", rgb[0], rgb[1], rgb[2], count, MatchRGBToNES(rgb[0], rgb[1], rgb[2]))
		}
	}

	inspect("font", "../../examples/data/font.pal", "../../examples/data/font.png")
	inspect("sprites", "../../examples/data/sprites.pal", "../../examples/data/sprites.png")
	inspect("tiles_surface", "../../game/src/data/gfx/tiles_surface.pal", "../../game/src/data/gfx/tiles_surface.png")
}
