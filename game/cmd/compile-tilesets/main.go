package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/qbradq/m3/pkg/compiler"
	"github.com/qbradq/m3/pkg/gfx"
)

// TilesetJSON represents the structure of a tileset definition JSON file.
type TilesetJSON struct {
	Name  string                   `json:"name"`
	Image string                   `json:"image"`
	Tiles []map[string]interface{} `json:"tiles"`
}

type Config struct {
	TilesetsDir   string
	TilesetM3Path string
	GfxDir        string
	OutputM3Dir   string
	OutputMapsDir string
}

func main() {
	cfg := parseFlags()
	if err := run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() Config {
	// Auto-detect default directories based on working directory
	tilesetsDir := "game/data/tilesets"
	tilesetM3 := "game/src/tileset.m3"
	gfxDir := "game/src/data/gfx"
	outputM3Dir := "game/src/data/tilesets"
	outputMapsDir := "game/data/maps"

	if _, err := os.Stat("data/tilesets"); err == nil {
		tilesetsDir = "data/tilesets"
		tilesetM3 = "src/tileset.m3"
		gfxDir = "src/data/gfx"
		outputM3Dir = "src/data/tilesets"
		outputMapsDir = "data/maps"
	}

	var cfg Config
	flag.StringVar(&cfg.TilesetsDir, "tilesets-dir", tilesetsDir, "Directory containing input tileset JSON files")
	flag.StringVar(&cfg.TilesetM3Path, "tileset-m3", tilesetM3, "Path to tileset.m3 defining the Tile struct")
	flag.StringVar(&cfg.GfxDir, "gfx-dir", gfxDir, "Directory containing source GFX and palette files")
	flag.StringVar(&cfg.OutputM3Dir, "output-m3-dir", outputM3Dir, "Directory to output compiled .m3 tileset files")
	flag.StringVar(&cfg.OutputMapsDir, "output-maps-dir", outputMapsDir, "Directory to output compiled tileset PNG maps")
	flag.Parse()

	return cfg
}

func run(cfg Config) error {
	tileFields, err := loadTileStructFields(cfg.TilesetM3Path)
	if err != nil {
		return fmt.Errorf("failed to load Tile struct from %q: %w", cfg.TilesetM3Path, err)
	}

	entries, err := os.ReadDir(cfg.TilesetsDir)
	if err != nil {
		return fmt.Errorf("cannot read tilesets directory %q: %w", cfg.TilesetsDir, err)
	}

	if err := os.MkdirAll(cfg.OutputM3Dir, 0755); err != nil {
		return fmt.Errorf("cannot create output m3 directory %q: %w", cfg.OutputM3Dir, err)
	}
	if err := os.MkdirAll(cfg.OutputMapsDir, 0755); err != nil {
		return fmt.Errorf("cannot create output maps directory %q: %w", cfg.OutputMapsDir, err)
	}

	var jsonFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			jsonFiles = append(jsonFiles, entry.Name())
		}
	}
	sort.Strings(jsonFiles)

	for _, filename := range jsonFiles {
		jsonPath := filepath.Join(cfg.TilesetsDir, filename)
		if err := processTilesetFile(jsonPath, tileFields, cfg); err != nil {
			return fmt.Errorf("error processing %s: %w", filename, err)
		}
	}

	return nil
}

func loadTileStructFields(tilesetM3Path string) ([]*compiler.StructField, error) {
	content, err := os.ReadFile(tilesetM3Path)
	if err != nil {
		return nil, err
	}

	lexer := compiler.NewLexer(tilesetM3Path, string(content))
	parser := compiler.NewParser(lexer)
	astFile, err := parser.ParseSourceFile()
	if err != nil {
		return nil, err
	}

	for _, decl := range astFile.Decls {
		if td, ok := decl.(*compiler.TypeDecl); ok && td.Name == "Tile" {
			if st, ok := td.Type.(*compiler.StructType); ok {
				return st.Fields, nil
			}
		}
	}

	return nil, fmt.Errorf("Tile struct definition not found in %s", tilesetM3Path)
}

func processTilesetFile(jsonPath string, tileFields []*compiler.StructField, cfg Config) error {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return fmt.Errorf("cannot read file %q: %w", jsonPath, err)
	}

	var tileset TilesetJSON
	if err := json.Unmarshal(data, &tileset); err != nil {
		return fmt.Errorf("cannot parse JSON: %w", err)
	}

	basename := strings.TrimSuffix(filepath.Base(jsonPath), filepath.Ext(jsonPath))

	// 1. Generate M3 struct literal array
	m3Content, err := generateM3Code(basename, &tileset, tileFields)
	if err != nil {
		return fmt.Errorf("failed to generate M3 code: %w", err)
	}

	outM3Path := filepath.Join(cfg.OutputM3Dir, basename+".m3")
	if err := os.WriteFile(outM3Path, []byte(m3Content), 0644); err != nil {
		return fmt.Errorf("cannot write M3 file %q: %w", outM3Path, err)
	}
	fmt.Printf("Generated %s\n", outM3Path)

	// 2. Generate tileset image
	if tileset.Image != "" {
		outImgPath := filepath.Join(cfg.OutputMapsDir, basename+".png")
		if err := generateTilesetImage(&tileset, cfg.GfxDir, outImgPath); err != nil {
			return fmt.Errorf("failed to generate tileset image: %w", err)
		}
		fmt.Printf("Generated %s\n", outImgPath)
	}

	return nil
}

func generateM3Code(basename string, tileset *TilesetJSON, tileFields []*compiler.StructField) (string, error) {
	var sb strings.Builder

	// Name array as basename with first character upper-cased, followed by "Tileset"
	var arrayName string
	if len(basename) > 0 {
		runes := []rune(basename)
		runes[0] = unicode.ToUpper(runes[0])
		arrayName = string(runes) + "Tileset"
	} else {
		arrayName = "Tileset"
	}

	sb.WriteString("package data\n\n")
	sb.WriteString("import \"../../tileset.m3\"\n\n")
	sb.WriteString(fmt.Sprintf("data %s Tile[] = {\n", arrayName))

	for _, tileMap := range tileset.Tiles {
		sb.WriteString("\t{\n")
		for _, field := range tileFields {
			valStr := formatFieldValue(field, tileMap)
			sb.WriteString(fmt.Sprintf("\t\t%s: %s,\n", field.Name, valStr))
		}
		sb.WriteString("\t},\n")
	}

	sb.WriteString("}\n")
	return sb.String(), nil
}

func formatFieldValue(field *compiler.StructField, tileMap map[string]interface{}) string {
	// Look up value case-insensitively and ignoring underscores (e.g. blocks_vis -> BlocksVis)
	var rawVal interface{}
	normField := strings.ToLower(strings.ReplaceAll(field.Name, "_", ""))
	for k, v := range tileMap {
		if strings.ToLower(strings.ReplaceAll(k, "_", "")) == normField {
			rawVal = v
			break
		}
	}

	switch field.Type.(type) {
	case *compiler.ArrayType:
		// Array of numbers (e.g. Chr uint8[4])
		var elements []string
		if list, ok := rawVal.([]interface{}); ok {
			for _, item := range list {
				if num, ok := item.(float64); ok {
					elements = append(elements, fmt.Sprintf("%d", int(num)))
				} else {
					elements = append(elements, fmt.Sprintf("%v", item))
				}
			}
		}
		if len(elements) == 0 {
			elements = []string{"0", "0", "0", "0"}
		}
		return "{" + strings.Join(elements, ", ") + "}"

	case *compiler.NamedType:
		named := field.Type.(*compiler.NamedType)
		if named.Name == "bool" {
			if b, ok := rawVal.(bool); ok {
				if b {
					return "true"
				}
				return "false"
			}
			return "false"
		}
		// Integer types: uint8, int8, etc.
		if num, ok := rawVal.(float64); ok {
			return fmt.Sprintf("%d", int(num))
		}
		return "0"

	default:
		if num, ok := rawVal.(float64); ok {
			return fmt.Sprintf("%d", int(num))
		}
		return "0"
	}
}

func generateTilesetImage(tileset *TilesetJSON, gfxDir string, outPath string) error {
	imagePath := filepath.Join(gfxDir, tileset.Image)
	imgFile, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("cannot open source image %q: %w", imagePath, err)
	}
	defer imgFile.Close()

	srcDecoded, _, err := image.Decode(imgFile)
	if err != nil {
		return fmt.Errorf("failed to decode source PNG %q: %w", imagePath, err)
	}

	bounds := srcDecoded.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width%8 != 0 || height%8 != 0 {
		return fmt.Errorf("source image %q dimensions (%dx%d) must be multiples of 8", imagePath, width, height)
	}

	tilesX := width / 8
	tilesY := height / 8
	totalChrTiles := tilesX * tilesY

	// Companion palette file
	palExt := filepath.Ext(tileset.Image)
	palBase := strings.TrimSuffix(tileset.Image, palExt)
	palPath := filepath.Join(gfxDir, palBase+".pal")

	pal, err := gfx.LoadPalFile(palPath)
	if err != nil {
		return fmt.Errorf("cannot load companion palette %q: %w", palPath, err)
	}

	// Seek back to start to convert to CHR with palette
	if _, err := imgFile.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek image file: %w", err)
	}

	chrData, err := gfx.ConvertPNGToCHRWithPalette(imgFile, pal)
	if err != nil {
		return fmt.Errorf("failed to convert PNG to CHR: %w", err)
	}

	// Build map of SubPalette by Slot
	subPalettes := make(map[int]*gfx.SubPalette)
	for i := range pal.Palettes {
		subPalettes[pal.Palettes[i].Slot] = &pal.Palettes[i]
	}

	numTiles := len(tileset.Tiles)
	numRows := (numTiles + 15) / 16
	if numRows == 0 {
		numRows = 1
	}

	// Create 256px wide (16 tiles across) canvas
	outImg := image.NewRGBA(image.Rect(0, 0, 256, numRows*16))

	for i, tileMap := range tileset.Tiles {
		tileCol := i % 16
		tileRow := i / 16
		tileDstX := tileCol * 16
		tileDstY := tileRow * 16

		// Extract palette slot
		palSlot := 0
		for k, v := range tileMap {
			if strings.EqualFold(k, "palette") {
				if num, ok := v.(float64); ok {
					palSlot = int(num)
				}
				break
			}
		}

		subPal := subPalettes[palSlot]
		if subPal == nil && len(pal.Palettes) > 0 {
			if palSlot >= 0 && palSlot < len(pal.Palettes) {
				subPal = &pal.Palettes[palSlot]
			} else {
				subPal = &pal.Palettes[0]
			}
		}

		// Extract CHR array [4]int
		chrIndices := [4]int{0, 0, 0, 0}
		for k, v := range tileMap {
			if strings.EqualFold(k, "chr") {
				if list, ok := v.([]interface{}); ok {
					for idx := 0; idx < 4 && idx < len(list); idx++ {
						if num, ok := list[idx].(float64); ok {
							chrIndices[idx] = int(num)
						}
					}
				}
				break
			}
		}

		// Quadrant offsets for 16x16 metatile (NW, NE, SW, SE)
		quadOffsets := [4][2]int{
			{0, 0}, // NW: chr[0]
			{8, 0}, // NE: chr[1]
			{0, 8}, // SW: chr[2]
			{8, 8}, // SE: chr[3]
		}

		for q := 0; q < 4; q++ {
			charIdx := chrIndices[q]
			chrTileIdx := charIdx % totalChrTiles
			chrOffset := chrTileIdx * 16
			tileBytes := chrData[chrOffset : chrOffset+16]

			qx := tileDstX + quadOffsets[q][0]
			qy := tileDstY + quadOffsets[q][1]

			for y := 0; y < 8; y++ {
				p0 := tileBytes[y]
				p1 := tileBytes[y+8]
				for x := 0; x < 8; x++ {
					bitPos := uint(7 - x)
					colIdx := ((p1 >> bitPos) & 1) << 1 | ((p0 >> bitPos) & 1)

					var nesCol byte = 0x0F
					if subPal != nil && int(colIdx) < len(subPal.Colors) {
						nesCol = subPal.Colors[colIdx]
					}
					rgb := gfx.NESPaletteRGB[nesCol]
					outImg.Set(qx+x, qy+y, color.RGBA{R: rgb[0], G: rgb[1], B: rgb[2], A: 255})
				}
			}
		}
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("cannot create output PNG %q: %w", outPath, err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, outImg); err != nil {
		return fmt.Errorf("failed to encode PNG: %w", err)
	}

	return nil
}
