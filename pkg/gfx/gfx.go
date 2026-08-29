package gfx

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"sort"
	"strings"
)

// NES 2C02 standard palette (64 colors in RGB)
var NESPaletteRGB = [64][3]uint8{
	// 0x00 - 0x0F
	{121, 121, 121}, {32, 0, 178}, {40, 0, 186}, {97, 16, 162},
	{154, 32, 121}, {178, 16, 48}, {162, 48, 0}, {121, 65, 0},
	{73, 89, 0}, {56, 105, 0}, {56, 109, 0}, {48, 97, 65},
	{48, 81, 130}, {0, 0, 0}, {0, 0, 0}, {0, 0, 0},

	// 0x10 - 0x1F
	{178, 178, 178}, {65, 97, 251}, {65, 65, 255}, {146, 65, 243},
	{219, 65, 195}, {219, 65, 97}, {227, 81, 0}, {195, 113, 0},
	{138, 138, 0}, {81, 162, 0}, {73, 170, 16}, {73, 162, 105},
	{65, 146, 195}, {0, 0, 0}, {0, 0, 0}, {0, 0, 0},

	// 0x20 - 0x2F
	{235, 235, 235}, {97, 162, 255}, {81, 130, 255}, {162, 113, 255},
	{243, 97, 255}, {255, 97, 178}, {255, 121, 48}, {255, 162, 0},
	{235, 211, 32}, {154, 235, 0}, {113, 243, 65}, {113, 227, 146},
	{97, 211, 227}, {121, 121, 121}, {0, 0, 0}, {0, 0, 0},

	// 0x30 - 0x3F
	{255, 255, 255}, {146, 211, 255}, {162, 186, 255}, {195, 178, 255},
	{227, 178, 255}, {255, 186, 235}, {255, 203, 186}, {255, 219, 162},
	{255, 243, 146}, {203, 243, 130}, {162, 243, 162}, {162, 255, 203},
	{162, 255, 243}, {162, 162, 162}, {0, 0, 0}, {0, 0, 0},
}

// MatchRGBToNES finds the closest NES 2C02 palette index (0x00 - 0x3F) for given R, G, B
func MatchRGBToNES(r, g, b uint8) byte {
	bestIndex := 0
	bestDist := int64(1<<60)

	for i := 0; i < 64; i++ {
		// Skip duplicate white entry $20 (only $30 is allowed)
		if i == 0x20 {
			continue
		}

		// Skip duplicate/forbidden black entries ($0D-$0F, $1D-$1F, $2D-$2F, $3D-$3F except standard black $0F)
		if (i >= 0x0D && i <= 0x0F) || (i >= 0x1D && i <= 0x1F) || (i >= 0x2D && i <= 0x2F) || (i >= 0x3D && i <= 0x3F) {
			if i != 0x0F {
				continue
			}
		}

		palR := int64(NESPaletteRGB[i][0])
		palG := int64(NESPaletteRGB[i][1])
		palB := int64(NESPaletteRGB[i][2])

		dr := int64(r) - palR
		dg := int64(g) - palG
		db := int64(b) - palB

		// Weighted Euclidean distance for perceptual color difference
		dist := 30*dr*dr + 59*dg*dg + 11*db*db
		if dist < bestDist {
			bestDist = dist
			bestIndex = i
		}
	}

	return byte(bestIndex)
}

// ColorLuminance computes perceived luminance of a color
func ColorLuminance(c color.Color) uint32 {
	r, g, b, _ := c.RGBA()
	return (r*299 + g*587 + b*114) / 1000
}

type uniqueColor struct {
	c     color.Color
	rgba  [4]uint8
	lum   uint32
	isTr  bool
	order int
}

// PixelToNESColor maps a single pixel color to an NES 2C02 palette index byte.
// Transparent pixels (alpha < 128) are mapped to 0x0F.
func PixelToNESColor(c color.Color) byte {
	r, g, b, a := c.RGBA()
	if a < 32768 {
		return 0x0F
	}
	return MatchRGBToNES(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// ConvertPNGToCHRWithPalette converts a PNG image stream into standard NES 2BPP planar CHR data
// using sub-palettes defined in the provided PaletteFile.
func ConvertPNGToCHRWithPalette(r io.Reader, pal *PaletteFile) ([]byte, error) {
	if pal == nil || len(pal.Palettes) == 0 {
		return nil, fmt.Errorf("no palettes defined in palette file")
	}

	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width%8 != 0 || height%8 != 0 {
		return nil, fmt.Errorf("PNG dimensions (%dx%d) must be multiples of 8", width, height)
	}

	tilesX := width / 8
	tilesY := height / 8
	totalTiles := tilesX * tilesY
	chrData := make([]byte, totalTiles*16)

	for ty := 0; ty < tilesY; ty++ {
		for tx := 0; tx < tilesX; tx++ {
			tileIdx := ty*tilesX + tx
			tileOffset := tileIdx * 16
			startX := bounds.Min.X + tx*8
			startY := bounds.Min.Y + ty*8

			// 1. Collect unique NES colors in this 8x8 tile
			colorSet := make(map[byte]bool)
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					c := img.At(startX+x, startY+y)
					nesCol := PixelToNESColor(c)
					colorSet[nesCol] = true
				}
			}

			// 2. Find a matching sub-palette from pal.Palettes
			var matchedPal *SubPalette
			for i := range pal.Palettes {
				sub := &pal.Palettes[i]
				allFound := true
				for nesCol := range colorSet {
					found := false
					for _, sc := range sub.Colors {
						if sc == nesCol {
							found = true
							break
						}
					}
					if !found {
						allFound = false
						break
					}
				}
				if allFound {
					matchedPal = sub
					break
				}
			}

			if matchedPal == nil {
				// Format offending color indexes
				colorsList := make([]byte, 0, len(colorSet))
				for nesCol := range colorSet {
					colorsList = append(colorsList, nesCol)
				}
				sort.Slice(colorsList, func(i, j int) bool { return colorsList[i] < colorsList[j] })
				var hexCols []string
				for _, c := range colorsList {
					hexCols = append(hexCols, fmt.Sprintf("$%02X", c))
				}
				return nil, fmt.Errorf("8x8 tile at (%d, %d) [tile #%d] cannot be placed in any palette; offending color indexes: %s",
					tx, ty, tileIdx, strings.Join(hexCols, ", "))
			}

			// 3. Encode 8x8 tile using the matched sub-palette
			out := chrData[tileOffset : tileOffset+16]
			for y := 0; y < 8; y++ {
				var p0, p1 byte
				for x := 0; x < 8; x++ {
					c := img.At(startX+x, startY+y)
					nesCol := PixelToNESColor(c)
					var colIdx byte
					for idx, sc := range matchedPal.Colors {
						if sc == nesCol {
							colIdx = byte(idx)
							break
						}
					}
					bitPos := uint(7 - x)
					p0 |= (colIdx & 1) << bitPos
					p1 |= ((colIdx >> 1) & 1) << bitPos
				}
				out[y] = p0
				out[y+8] = p1
			}
		}
	}

	return chrData, nil
}

// ConvertPNGToCHR converts a PNG image stream into standard NES 2BPP planar CHR data (16 bytes per 8x8 tile).
func ConvertPNGToCHR(r io.Reader) ([]byte, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width%8 != 0 || height%8 != 0 {
		return nil, fmt.Errorf("PNG dimensions (%dx%d) must be multiples of 8", width, height)
	}

	tilesX := width / 8
	tilesY := height / 8
	totalTiles := tilesX * tilesY
	chrData := make([]byte, totalTiles*16)

	// If paletted image with <= 4 colors, use palette indices directly (mod 4)
	if palImg, ok := img.(*image.Paletted); ok && len(palImg.Palette) <= 4 {
		for ty := 0; ty < tilesY; ty++ {
			for tx := 0; tx < tilesX; tx++ {
				tileIdx := ty*tilesX + tx
				tileOffset := tileIdx * 16
				encodeTilePaletted(palImg, bounds.Min.X+tx*8, bounds.Min.Y+ty*8, chrData[tileOffset:tileOffset+16])
			}
		}
		return chrData, nil
	}

	// For general images: extract unique colors and map to 0..3 indices
	colorMap := buildColorIndexMap(img)

	for ty := 0; ty < tilesY; ty++ {
		for tx := 0; tx < tilesX; tx++ {
			tileIdx := ty*tilesX + tx
			tileOffset := tileIdx * 16
			encodeTileGeneral(img, bounds.Min.X+tx*8, bounds.Min.Y+ty*8, colorMap, chrData[tileOffset:tileOffset+16])
		}
	}

	return chrData, nil
}

func encodeTilePaletted(img *image.Paletted, startX, startY int, out []byte) {
	for y := 0; y < 8; y++ {
		var p0, p1 byte
		for x := 0; x < 8; x++ {
			colIdx := img.ColorIndexAt(startX+x, startY+y) % 4
			bitPos := uint(7 - x)
			p0 |= (colIdx & 1) << bitPos
			p1 |= ((colIdx >> 1) & 1) << bitPos
		}
		out[y] = p0
		out[y+8] = p1
	}
}

func encodeTileGeneral(img image.Image, startX, startY int, colorMap map[[4]uint8]byte, out []byte) {
	for y := 0; y < 8; y++ {
		var p0, p1 byte
		for x := 0; x < 8; x++ {
			r, g, b, a := img.At(startX+x, startY+y).RGBA()
			key := [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
			colIdx := colorMap[key] % 4
			bitPos := uint(7 - x)
			p0 |= (colIdx & 1) << bitPos
			p1 |= ((colIdx >> 1) & 1) << bitPos
		}
		out[y] = p0
		out[y+8] = p1
	}
}

func buildColorIndexMap(img image.Image) map[[4]uint8]byte {
	bounds := img.Bounds()
	unique := make(map[[4]uint8]*uniqueColor)
	order := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()
			key := [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
			if _, exists := unique[key]; !exists {
				unique[key] = &uniqueColor{
					c:     c,
					rgba:  key,
					lum:   ColorLuminance(c),
					isTr:  key[3] < 128,
					order: order,
				}
				order++
			}
		}
	}

	colors := make([]*uniqueColor, 0, len(unique))
	for _, uc := range unique {
		colors = append(colors, uc)
	}

	// Sort colors: transparent first, then by luminance ascending (dark to bright)
	sort.Slice(colors, func(i, j int) bool {
		if colors[i].isTr != colors[j].isTr {
			return colors[i].isTr // transparent comes first (index 0)
		}
		if colors[i].lum != colors[j].lum {
			return colors[i].lum < colors[j].lum
		}
		return colors[i].order < colors[j].order
	})

	result := make(map[[4]uint8]byte)
	for i, uc := range colors {
		result[uc.rgba] = byte(i % 4)
	}
	return result
}

// ExtractPNGPalette extracts colors from a PNG image and maps them to NES 2C02 palette indices ($00-$3F).
func ExtractPNGPalette(r io.Reader, maxColors int) ([]byte, error) {
	if maxColors <= 0 {
		maxColors = 4
	}

	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG: %w", err)
	}

	paletteBytes := make([]byte, maxColors)
	// Default fill with NES black ($0F)
	for i := range paletteBytes {
		paletteBytes[i] = 0x0F
	}

	if palImg, ok := img.(*image.Paletted); ok && len(palImg.Palette) > 0 {
		for i := 0; i < len(palImg.Palette) && i < maxColors; i++ {
			c := palImg.Palette[i]
			r, g, b, a := c.RGBA()
			if a < 32768 {
				paletteBytes[i] = 0x0F // Black / transparent
			} else {
				paletteBytes[i] = MatchRGBToNES(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			}
		}
		return paletteBytes, nil
	}

	// For general images: collect unique colors
	bounds := img.Bounds()
	unique := make(map[[4]uint8]*uniqueColor)
	order := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r, g, b, a := c.RGBA()
			key := [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
			if _, exists := unique[key]; !exists {
				unique[key] = &uniqueColor{
					c:     c,
					rgba:  key,
					lum:   ColorLuminance(c),
					isTr:  key[3] < 128,
					order: order,
				}
				order++
			}
		}
	}

	colors := make([]*uniqueColor, 0, len(unique))
	for _, uc := range unique {
		colors = append(colors, uc)
	}

	sort.Slice(colors, func(i, j int) bool {
		if colors[i].isTr != colors[j].isTr {
			return colors[i].isTr
		}
		if colors[i].lum != colors[j].lum {
			return colors[i].lum < colors[j].lum
		}
		return colors[i].order < colors[j].order
	})

	for i := 0; i < len(colors) && i < maxColors; i++ {
		if colors[i].isTr {
			paletteBytes[i] = 0x0F
		} else {
			paletteBytes[i] = MatchRGBToNES(colors[i].rgba[0], colors[i].rgba[1], colors[i].rgba[2])
		}
	}

	return paletteBytes, nil
}
