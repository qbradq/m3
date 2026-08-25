package gfx

import (
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"sort"
)

// NES 2C02 standard palette (64 colors in RGB)
var NESPaletteRGB = [64][3]uint8{
	// 0x00 - 0x0F
	{84, 84, 84}, {0, 30, 116}, {8, 16, 144}, {48, 0, 136},
	{68, 0, 100}, {92, 0, 48}, {84, 4, 0}, {60, 24, 0},
	{32, 42, 0}, {8, 58, 0}, {0, 64, 0}, {0, 60, 0},
	{0, 50, 60}, {0, 0, 0}, {0, 0, 0}, {0, 0, 0},

	// 0x10 - 0x1F
	{152, 150, 152}, {8, 76, 196}, {48, 50, 236}, {92, 30, 228},
	{136, 20, 176}, {160, 20, 100}, {152, 34, 32}, {120, 60, 0},
	{84, 90, 0}, {40, 114, 0}, {8, 124, 0}, {0, 118, 40},
	{0, 102, 120}, {0, 0, 0}, {0, 0, 0}, {0, 0, 0},

	// 0x20 - 0x2F
	{236, 238, 236}, {76, 154, 236}, {120, 124, 236}, {176, 98, 236},
	{228, 84, 236}, {236, 88, 180}, {236, 106, 100}, {212, 136, 32},
	{160, 170, 0}, {116, 196, 0}, {76, 208, 32}, {56, 204, 108},
	{56, 180, 204}, {60, 60, 60}, {0, 0, 0}, {0, 0, 0},

	// 0x30 - 0x3F
	{236, 238, 236}, {168, 204, 236}, {188, 188, 236}, {212, 178, 236},
	{236, 174, 236}, {236, 174, 212}, {236, 180, 176}, {228, 196, 144},
	{204, 210, 120}, {180, 222, 120}, {168, 226, 144}, {152, 226, 180},
	{160, 214, 228}, {160, 162, 160}, {0, 0, 0}, {0, 0, 0},
}

// MatchRGBToNES finds the closest NES 2C02 palette index (0x00 - 0x3F) for given R, G, B
func MatchRGBToNES(r, g, b uint8) byte {
	bestIndex := 0
	bestDist := int64(1<<60)

	for i := 0; i < 64; i++ {
		// Skip duplicate black entries
		if (i >= 0x0E && i <= 0x0F) || (i >= 0x1E && i <= 0x1F) || (i >= 0x2E && i <= 0x2F) || (i >= 0x3E && i <= 0x3F) {
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
