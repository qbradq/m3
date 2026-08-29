package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qbradq/m3/pkg/compiler"
)

func TestLoadTileStructFields(t *testing.T) {
	fields, err := loadTileStructFields("../../src/tileset.m3")
	if err != nil {
		t.Fatalf("loadTileStructFields failed: %v", err)
	}

	expected := map[string]string{
		"Chr":       "uint8[4]",
		"Palette":   "uint8",
		"BlocksVis": "bool",
		"Walkable":  "bool",
		"Sailable":  "bool",
	}

	if len(fields) != len(expected) {
		t.Fatalf("expected %d fields, got %d", len(expected), len(fields))
	}

	for _, f := range fields {
		if _, ok := expected[f.Name]; !ok {
			t.Errorf("unexpected field %q in Tile struct", f.Name)
		}
	}
}

func TestGenerateM3Code(t *testing.T) {
	fields, err := loadTileStructFields("../../src/tileset.m3")
	if err != nil {
		t.Fatalf("loadTileStructFields failed: %v", err)
	}

	tileset := &TilesetJSON{
		Name:  "Surface",
		Image: "tiles_surface.png",
		Tiles: []map[string]interface{}{
			{
				"name":       "Deep Water",
				"chr":        []interface{}{float64(128), float64(128), float64(128), float64(128)},
				"palette":    float64(1),
				"blocks_vis": false,
				"walkable":   false,
				"sailable":   true,
			},
			{
				"name":       "Grass",
				"chr":        []interface{}{float64(130), float64(130), float64(130), float64(130)},
				"palette":    float64(2),
				"blocks_vis": false,
				"walkable":   true,
				"sailable":   false,
			},
		},
	}

	code, err := generateM3Code("surface", tileset, fields)
	if err != nil {
		t.Fatalf("generateM3Code failed: %v", err)
	}

	if !strings.Contains(code, "data SurfaceTileset Tile[] = {") {
		t.Errorf("expected 'data SurfaceTileset Tile[] = {', got:\n%s", code)
	}
	if !strings.Contains(code, "Chr: {128, 128, 128, 128}") {
		t.Errorf("expected Chr: {128, 128, 128, 128}, got:\n%s", code)
	}
	if !strings.Contains(code, "Palette: 1") {
		t.Errorf("expected Palette: 1, got:\n%s", code)
	}
	if !strings.Contains(code, "BlocksVis: false") {
		t.Errorf("expected BlocksVis: false, got:\n%s", code)
	}
	if !strings.Contains(code, "Walkable: false") {
		t.Errorf("expected Walkable: false, got:\n%s", code)
	}
	if !strings.Contains(code, "Sailable: true") {
		t.Errorf("expected Sailable: true, got:\n%s", code)
	}
	// "name" should be omitted
	if strings.Contains(code, "Deep Water") || strings.Contains(code, "Name:") {
		t.Errorf("extraneous field 'name' should not be present in generated code:\n%s", code)
	}

	// Verify that the generated M3 code compiles successfully when combined with Tile struct
	fullSource := `package data

type Tile struct {
	Chr uint8[4]
	Palette uint8
	BlocksVis bool
	Walkable bool
	Sailable bool
}

` + strings.Replace(code, "package data\n\nimport \"../../tileset.m3\"\n\n", "", 1)

	_, asmCode, err := compiler.Compile("surface.m3", fullSource)
	if err != nil {
		t.Fatalf("failed to compile generated M3 code: %v", err)
	}

	if !strings.Contains(asmCode, "_data_SurfaceTileset:") {
		t.Errorf("expected symbol _data_SurfaceTileset in assembly output, got:\n%s", asmCode)
	}
}

func TestGenerateTilesetImage(t *testing.T) {
	tempDir := t.TempDir()
	outImgPath := filepath.Join(tempDir, "surface.png")

	tileset := &TilesetJSON{
		Name:  "Surface",
		Image: "tiles_surface.png",
		Tiles: []map[string]interface{}{
			{
				"name":     "Deep Water",
				"chr":      []interface{}{float64(128), float64(128), float64(128), float64(128)},
				"palette":  float64(1),
				"walkable": false,
				"sailable": true,
			},
			{
				"name":     "Grass",
				"chr":      []interface{}{float64(130), float64(130), float64(130), float64(130)},
				"palette":  float64(2),
				"walkable": true,
				"sailable": false,
			},
		},
	}

	err := generateTilesetImage(tileset, "../../src/data/gfx", outImgPath)
	if err != nil {
		t.Fatalf("generateTilesetImage failed: %v", err)
	}

	f, err := os.Open(outImgPath)
	if err != nil {
		t.Fatalf("failed to open generated PNG: %v", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("failed to decode generated PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 256 {
		t.Errorf("expected width 256, got %d", bounds.Dx())
	}
	if bounds.Dy() != 16 {
		t.Errorf("expected height 16, got %d", bounds.Dy())
	}
}

func TestRunEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	outM3Dir := filepath.Join(tempDir, "m3")
	outMapsDir := filepath.Join(tempDir, "maps")

	cfg := Config{
		TilesetsDir:   "../../data/tilesets",
		TilesetM3Path: "../../src/tileset.m3",
		GfxDir:        "../../src/data/gfx",
		OutputM3Dir:   outM3Dir,
		OutputMapsDir: outMapsDir,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify surface.m3 generated
	m3Path := filepath.Join(outM3Dir, "surface.m3")
	if _, err := os.Stat(m3Path); err != nil {
		t.Fatalf("surface.m3 not found at %s: %v", m3Path, err)
	}

	// Verify surface.png generated
	pngPath := filepath.Join(outMapsDir, "surface.png")
	if _, err := os.Stat(pngPath); err != nil {
		t.Fatalf("surface.png not found at %s: %v", pngPath, err)
	}

	imgFile, err := os.Open(pngPath)
	if err != nil {
		t.Fatalf("open PNG failed: %v", err)
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	if err != nil {
		t.Fatalf("decode PNG failed: %v", err)
	}

	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 16 {
		t.Errorf("unexpected image bounds: %v", img.Bounds())
	}
}
