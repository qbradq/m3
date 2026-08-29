package gfx

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	slotHeaderRegex = regexp.MustCompile(`^[0-3]:$`)
	colorHexRegex   = regexp.MustCompile(`^\$([0-9A-Fa-f]{1,2})$`)
)

// SubPalette represents a single 4-color palette slot for the NES (slots 0-3).
type SubPalette struct {
	Slot   int
	Colors []byte
}

// PaletteFile represents the parsed content of a .pal file.
type PaletteFile struct {
	Palettes []SubPalette
}

// ParsePal parses a .pal file stream.
func ParsePal(r io.Reader) (*PaletteFile, error) {
	scanner := bufio.NewScanner(r)
	palFile := &PaletteFile{}
	lastSlot := -1
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Remove inline comments starting with ';' or '#'
		if idx := strings.IndexAny(line, ";#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}

		if line == "" {
			continue
		}

		if slotHeaderRegex.MatchString(line) {
			slot := int(line[0] - '0')
			if slot <= lastSlot {
				return nil, fmt.Errorf("line %d: palettes out of numeric order: slot %d follows slot %d", lineNum, slot, lastSlot)
			}
			if len(palFile.Palettes) >= 4 {
				return nil, fmt.Errorf("line %d: more than 4 total palettes provided", lineNum)
			}
			lastSlot = slot
			palFile.Palettes = append(palFile.Palettes, SubPalette{
				Slot:   slot,
				Colors: make([]byte, 0, 4),
			})
			continue
		}

		if len(palFile.Palettes) == 0 {
			return nil, fmt.Errorf("line %d: color defined before palette slot header: %q", lineNum, line)
		}

		currPal := &palFile.Palettes[len(palFile.Palettes)-1]
		if len(currPal.Colors) >= 4 {
			return nil, fmt.Errorf("line %d: palette %d contains more than 4 colors", lineNum, currPal.Slot)
		}

		matches := colorHexRegex.FindStringSubmatch(line)
		if len(matches) < 2 {
			return nil, fmt.Errorf("line %d: invalid palette line %q, expected slot header (e.g. 0:) or hex color (e.g. $0F)", lineNum, line)
		}

		val, err := strconv.ParseUint(matches[1], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid hex color value %q: %w", lineNum, line, err)
		}

		colorByte := byte(val)
		if colorByte == 0x0D {
			return nil, fmt.Errorf("line %d: palette references forbidden color $0D", lineNum)
		}
		if colorByte == 0x20 {
			return nil, fmt.Errorf("line %d: palette references forbidden color $20", lineNum)
		}
		if colorByte > 0x3F {
			return nil, fmt.Errorf("line %d: color $%02X out of NES palette range ($00-$3F)", lineNum, colorByte)
		}

		currPal.Colors = append(currPal.Colors, colorByte)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading palette: %w", err)
	}

	return palFile, nil
}

// LoadPalFile reads and parses a .pal file from disk.
func LoadPalFile(path string) (*PaletteFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open palette file %q: %w", path, err)
	}
	defer f.Close()

	pal, err := ParsePal(f)
	if err != nil {
		return nil, fmt.Errorf("cannot parse palette file %q: %w", path, err)
	}
	return pal, nil
}

// ToBytes converts the palette file to raw binary bytes in slot and color order.
func (p *PaletteFile) ToBytes() []byte {
	var out []byte
	for _, sub := range p.Palettes {
		out = append(out, sub.Colors...)
	}
	return out
}
