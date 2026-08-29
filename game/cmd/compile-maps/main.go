package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// MapXML represents the root <map> element of a Tiled TMX file.
type MapXML struct {
	XMLName  xml.Name     `xml:"map"`
	Width    int          `xml:"width,attr"`
	Height   int          `xml:"height,attr"`
	Tilesets []TilesetXML `xml:"tileset"`
	Layers   []LayerXML   `xml:"layer"`
}

// TilesetXML represents a <tileset> reference in a TMX file.
type TilesetXML struct {
	FirstGID int    `xml:"firstgid,attr"`
	Source   string `xml:"source,attr"`
}

// LayerXML represents a <layer> in a TMX file.
type LayerXML struct {
	ID     int     `xml:"id,attr"`
	Name   string  `xml:"name,attr"`
	Width  int     `xml:"width,attr"`
	Height int     `xml:"height,attr"`
	Data   DataXML `xml:"data"`
}

// DataXML represents the <data> block in a layer.
type DataXML struct {
	Encoding string    `xml:"encoding,attr"`
	Value    string    `xml:",chardata"`
	Tiles    []TileXML `xml:"tile"`
}

// TileXML represents a <tile> element in XML data.
type TileXML struct {
	GID uint32 `xml:"gid,attr"`
}

// Config contains runtime configuration for the compile-maps tool.
type Config struct {
	MapsDir   string
	OutputDir string
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() Config {
	mapsDir := "game/data/maps"
	outputDir := "game/src/data/maps"

	if _, err := os.Stat("data/maps"); err == nil {
		mapsDir = "data/maps"
		outputDir = "src/data/maps"
	}

	var cfg Config
	flag.StringVar(&cfg.MapsDir, "maps-dir", mapsDir, "Directory containing input Tiled TMX map files")
	flag.StringVar(&cfg.OutputDir, "output-dir", outputDir, "Directory to output compiled binary map files")
	flag.Parse()

	return cfg
}

func run(cfg Config) error {
	entries, err := os.ReadDir(cfg.MapsDir)
	if err != nil {
		return fmt.Errorf("cannot read maps directory %q: %w", cfg.MapsDir, err)
	}

	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("cannot create output directory %q: %w", cfg.OutputDir, err)
	}

	var tmxFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".tmx") {
			tmxFiles = append(tmxFiles, entry.Name())
		}
	}
	sort.Strings(tmxFiles)

	for _, filename := range tmxFiles {
		tmxPath := filepath.Join(cfg.MapsDir, filename)
		basename := strings.TrimSuffix(filename, filepath.Ext(filename))
		outPath := filepath.Join(cfg.OutputDir, basename+".bin")

		if err := compileMapFile(tmxPath, outPath); err != nil {
			return fmt.Errorf("error processing %s: %w", filename, err)
		}
		fmt.Printf("Generated %s\n", outPath)
	}

	return nil
}

func compileMapFile(tmxPath, outPath string) error {
	data, err := os.ReadFile(tmxPath)
	if err != nil {
		return fmt.Errorf("cannot read file %q: %w", tmxPath, err)
	}

	mapBytes, err := ParseTMXMap(data)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outPath, mapBytes, 0644); err != nil {
		return fmt.Errorf("cannot write output file %q: %w", outPath, err)
	}

	return nil
}

// ParseTMXMap parses raw XML data of a TMX map file, verifies that it is 64x64,
// and extracts the 0-based tile index 4096-byte array.
func ParseTMXMap(data []byte) ([]byte, error) {
	var m MapXML
	if err := xml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse TMX XML: %w", err)
	}

	if m.Width != 64 || m.Height != 64 {
		return nil, fmt.Errorf("map dimensions (%dx%d) must be 64x64", m.Width, m.Height)
	}

	if len(m.Layers) == 0 {
		return nil, fmt.Errorf("no layer found in map")
	}

	layer := m.Layers[0]
	if layer.Width != 0 && layer.Height != 0 {
		if layer.Width != 64 || layer.Height != 64 {
			return nil, fmt.Errorf("layer dimensions (%dx%d) must be 64x64", layer.Width, layer.Height)
		}
	}

	firstGID := 1
	if len(m.Tilesets) > 0 && m.Tilesets[0].FirstGID > 0 {
		firstGID = m.Tilesets[0].FirstGID
	}

	var gids []uint32

	if len(layer.Data.Tiles) > 0 {
		for _, t := range layer.Data.Tiles {
			gids = append(gids, t.GID)
		}
	} else {
		text := strings.TrimSpace(layer.Data.Value)
		if text == "" {
			return nil, fmt.Errorf("empty map data in layer %q", layer.Name)
		}
		// Split by comma or whitespace
		tokens := strings.FieldsFunc(text, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
		})
		for _, tok := range tokens {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			val, err := strconv.ParseUint(tok, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid tile GID %q: %w", tok, err)
			}
			gids = append(gids, uint32(val))
		}
	}

	const expectedCount = 64 * 64
	if len(gids) != expectedCount {
		return nil, fmt.Errorf("expected %d tiles for 64x64 map, got %d", expectedCount, len(gids))
	}

	out := make([]byte, expectedCount)
	for i, rawGID := range gids {
		// Mask out Tiled flip flags (bits 31, 30, 29, 28)
		gid := rawGID & 0x1FFFFFFF
		var tileIndex uint32
		if gid == 0 {
			tileIndex = 0
		} else if gid >= uint32(firstGID) {
			tileIndex = gid - uint32(firstGID)
		} else {
			tileIndex = gid
		}

		if tileIndex > 255 {
			return nil, fmt.Errorf("tile at index %d has tile index %d which exceeds 255", i, tileIndex)
		}
		out[i] = byte(tileIndex)
	}

	return out, nil
}
