package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generateValidTMX(width, height int, firstGID int, tileValues []int) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" tiledversion="1.12.2" orientation="orthogonal" renderorder="right-down" width="%d" height="%d" tilewidth="16" tileheight="16" infinite="0">
 <tileset firstgid="%d" source="surface.tsx"/>
 <layer id="1" name="Tile Layer 1" width="%d" height="%d">
  <data encoding="csv">
`, width, height, firstGID, width, height))

	for i, v := range tileValues {
		if i > 0 {
			sb.WriteString(",")
			if i%64 == 0 {
				sb.WriteString("\n")
			}
		}
		sb.WriteString(fmt.Sprintf("%d", v))
	}
	sb.WriteString("\n</data>\n </layer>\n</map>\n")
	return sb.String()
}

func TestParseTMXMapValid(t *testing.T) {
	tiles := make([]int, 64*64)
	for i := range tiles {
		tiles[i] = (i % 16) + 1 // 1-based GIDs (1..16) with firstgid=1 -> 0-based (0..15)
	}

	tmxXML := generateValidTMX(64, 64, 1, tiles)
	data, err := ParseTMXMap([]byte(tmxXML))
	if err != nil {
		t.Fatalf("ParseTMXMap failed: %v", err)
	}

	if len(data) != 64*64 {
		t.Fatalf("expected %d bytes, got %d", 64*64, len(data))
	}

	for i, b := range data {
		expected := byte(i % 16)
		if b != expected {
			t.Errorf("tile[%d] = %d, want %d", i, b, expected)
		}
	}
}

func TestParseTMXMapDimensionsError(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"Too small", 32, 32},
		{"Non-square width", 64, 32},
		{"Non-square height", 32, 64},
		{"Too large", 128, 128},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tiles := make([]int, tc.width*tc.height)
			for i := range tiles {
				tiles[i] = 1
			}
			tmxXML := generateValidTMX(tc.width, tc.height, 1, tiles)
			_, err := ParseTMXMap([]byte(tmxXML))
			if err == nil {
				t.Fatalf("expected error for dimensions %dx%d, got nil", tc.width, tc.height)
			}
			if !strings.Contains(err.Error(), "64x64") {
				t.Errorf("expected error mentioning 64x64, got: %v", err)
			}
		})
	}
}

func TestParseTMXMapTileCountMismatch(t *testing.T) {
	tmxXML := `<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" width="64" height="64" tilewidth="16" tileheight="16">
 <tileset firstgid="1" source="surface.tsx"/>
 <layer id="1" name="Tile Layer 1" width="64" height="64">
  <data encoding="csv">
1,2,3,4
  </data>
 </layer>
</map>`

	_, err := ParseTMXMap([]byte(tmxXML))
	if err == nil {
		t.Fatalf("expected tile count error, got nil")
	}
	if !strings.Contains(err.Error(), "expected 4096 tiles") {
		t.Errorf("expected tile count mismatch error, got: %v", err)
	}
}

func TestParseTMXMapTileXMLFormat(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<map version="1.10" width="64" height="64" tilewidth="16" tileheight="16">
 <tileset firstgid="1" source="surface.tsx"/>
 <layer id="1" name="Tile Layer 1" width="64" height="64">
  <data>
`)
	for i := 0; i < 64*64; i++ {
		sb.WriteString(fmt.Sprintf(`<tile gid="%d"/>`, (i%5)+1))
	}
	sb.WriteString(`
  </data>
 </layer>
</map>`)

	data, err := ParseTMXMap([]byte(sb.String()))
	if err != nil {
		t.Fatalf("ParseTMXMap with <tile> elements failed: %v", err)
	}

	if len(data) != 4096 {
		t.Fatalf("expected 4096 bytes, got %d", len(data))
	}

	for i, b := range data {
		expected := byte(i % 5)
		if b != expected {
			t.Errorf("tile[%d] = %d, want %d", i, b, expected)
		}
	}
}

func TestRunEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	outDir := filepath.Join(tempDir, "maps_out")

	cfg := Config{
		MapsDir:   "../../data/maps",
		OutputDir: outDir,
	}

	if err := run(cfg); err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// Verify castle_brad.bin was generated
	binPath := filepath.Join(outDir, "castle_brad.bin")
	data, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatalf("failed to read generated binary map %q: %v", binPath, err)
	}

	if len(data) != 4096 {
		t.Fatalf("expected 4096 bytes, got %d", len(data))
	}

	// castle_brad.tmx first row starts with 7,5,6,5,7,2,2,3,3,3 (1-based GIDs)
	// which maps to 0-based indices 6,4,5,4,6,1,1,2,2,2
	expectedPrefix := []byte{6, 4, 5, 4, 6, 1, 1, 2, 2, 2}
	for i, exp := range expectedPrefix {
		if data[i] != exp {
			t.Errorf("data[%d] = %d, want %d", i, data[i], exp)
		}
	}
}
