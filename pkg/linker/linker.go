package linker

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/qbradq/m3/pkg/obj"
)

const (
	BankSize        = 8192 // 8KB MMC3 PRG bank
	NumBanks        = 64   // 512KB PRG ROM
	PRGROMSize      = NumBanks * BankSize
	INESHeaderSize  = 16
	TotalOutputSize = INESHeaderSize + PRGROMSize

	Bank0BaseAddr  = 0x8000
	Bank62BaseAddr = 0xC000
	Bank63BaseAddr = 0xE000

	// RAM Segment base addresses and capacity limits
	ZPBaseAddr   = 0x0000
	ZPMaxSize    = 0x0100 // 256 bytes ($0000 - $00FF)

	RAMBaseAddr  = 0x0300
	RAMMaxSize   = 0x0500 // 1280 bytes ($0300 - $07FF)

	WRAMBaseAddr = 0x6000
	WRAMMaxSize  = 0x2000 // 8192 bytes ($6000 - $7FFF)

	// Vector offsets within Bank 63 ($E000-$FFFF)
	NMIVectorOffset   = 0x1FFA // $FFFA
	ResetVectorOffset = 0x1FFC // $FFFC
	IRQVectorOffset   = 0x1FFE // $FFFE
)

// Config represents optional linker settings.
type Config struct {
	PRGBankCount int
	Mapper       uint8
	Mirroring    uint8 // 0: Horizontal/Mapper-controlled, 1: Vertical
	HasBattery   bool
}

// DefaultConfig returns the standard MMC3 configuration for m3.
func DefaultConfig() Config {
	return Config{
		PRGBankCount: NumBanks,
		Mapper:       4, // MMC3
		Mirroring:    0,
		HasBattery:   false,
	}
}

type symbolKey struct {
	fileIdx int
	name    string
}

type resolvedSymbol struct {
	Name         string
	Type         obj.SymbolType
	Scope        obj.SymbolScope
	Bank         int32
	OffsetInBank uint32
	Address      int64
	FileIndex    int
	SourceFile   string
}

type chunkPlacement struct {
	fileIndex   int
	bankIndex   uint32
	startOffset uint32
	chunk       *obj.BankChunk
}

// Linker coordinates the linking of multiple object files into a single NES ROM.
type Linker struct {
	config     Config
	objects    []*obj.ObjectFile
	banks      [NumBanks][]byte
	placements []chunkPlacement
	globalSyms map[string]*resolvedSymbol
	localSyms  map[symbolKey]*resolvedSymbol
}

// NewLinker creates a new Linker instance.
func NewLinker(objects ...*obj.ObjectFile) *Linker {
	return &Linker{
		config:     DefaultConfig(),
		objects:    objects,
		globalSyms: make(map[string]*resolvedSymbol),
		localSyms:  make(map[symbolKey]*resolvedSymbol),
	}
}

// Link performs linking and returns the complete .nes file bytes.
func (l *Linker) Link() ([]byte, error) {
	if len(l.objects) == 0 {
		return nil, fmt.Errorf("linker error: no object files provided")
	}

	// 1. Initialize bank buffers
	for i := 0; i < NumBanks; i++ {
		l.banks[i] = make([]byte, BankSize)
	}

	// 2. Place chunks and record chunk base offsets
	bankUsage := make([]uint32, NumBanks)
	for fileIdx, objFile := range l.objects {
		for _, chunk := range objFile.Banks {
			if chunk.BankIndex >= NumBanks {
				return nil, fmt.Errorf("linker error: %s has bank index %d exceeding maximum supported %d",
					objFile.SourceFile, chunk.BankIndex, NumBanks-1)
			}
			bIdx := chunk.BankIndex
			startOffset := bankUsage[bIdx]
			chunkLen := uint32(len(chunk.Data))

			if startOffset+chunkLen > BankSize {
				return nil, fmt.Errorf("linker error: bank %d overflow in %s (size %d bytes exceeds bank limit of %d bytes)",
					bIdx, objFile.SourceFile, startOffset+chunkLen, BankSize)
			}

			// Copy chunk data
			copy(l.banks[bIdx][startOffset:], chunk.Data)
			bankUsage[bIdx] += chunkLen

			l.placements = append(l.placements, chunkPlacement{
				fileIndex:   fileIdx,
				bankIndex:   bIdx,
				startOffset: startOffset,
				chunk:       chunk,
			})
		}
	}

	// 3. Build symbol tables and compute final addresses
	if err := l.collectSymbols(); err != nil {
		return nil, err
	}

	// 4. Resolve relocations
	if err := l.applyRelocations(); err != nil {
		return nil, err
	}

	// 5. Populate interrupt vectors in Bank 63
	if err := l.setupVectors(); err != nil {
		return nil, err
	}

	// 6. Generate iNES ROM image
	return l.buildINESImage(), nil
}

func (l *Linker) getChunkOffset(fileIdx int, bankIdx uint32) uint32 {
	for _, p := range l.placements {
		if p.fileIndex == fileIdx && p.bankIndex == bankIdx {
			return p.startOffset
		}
	}
	return 0
}

func BankBaseAddress(bankIndex uint32) int64 {
	switch bankIndex {
	case 0:
		return Bank0BaseAddr // $8000
	case NumBanks - 2: // Bank 62
		return Bank62BaseAddr // $C000
	case NumBanks - 1: // Bank 63
		return Bank63BaseAddr // $E000
	default:
		return 0x0000
	}
}

func (l *Linker) collectSymbols() error {
	fileZPBase := make([]uint32, len(l.objects))
	fileRAMBase := make([]uint32, len(l.objects))
	fileWRAMBase := make([]uint32, len(l.objects))

	var currentZPUsage uint32
	var currentRAMUsage uint32
	var currentWRAMUsage uint32

	// First calculate RAM segment sizes for each object file
	for fileIdx, objFile := range l.objects {
		var fileZPSize uint32
		var fileRAMSize uint32
		var fileWRAMSize uint32

		for _, sym := range objFile.Symbols {
			switch sym.Name {
			case "__seg_zp_size__":
				fileZPSize = uint32(sym.Value)
			case "__seg_ram_size__":
				fileRAMSize = uint32(sym.Value)
			case "__seg_wram_size__":
				fileWRAMSize = uint32(sym.Value)
			default:
				if sym.Bank == obj.BankZP && uint32(sym.Value+1) > fileZPSize {
					fileZPSize = uint32(sym.Value + 1)
				}
				if sym.Bank == obj.BankRAM && uint32(sym.Value+1) > fileRAMSize {
					fileRAMSize = uint32(sym.Value + 1)
				}
				if sym.Bank == obj.BankWRAM && uint32(sym.Value+1) > fileWRAMSize {
					fileWRAMSize = uint32(sym.Value + 1)
				}
			}
		}

		fileZPBase[fileIdx] = currentZPUsage
		currentZPUsage += fileZPSize

		fileRAMBase[fileIdx] = currentRAMUsage
		currentRAMUsage += fileRAMSize

		fileWRAMBase[fileIdx] = currentWRAMUsage
		currentWRAMUsage += fileWRAMSize
	}

	// Validate RAM segment bounds
	if currentZPUsage > ZPMaxSize {
		return fmt.Errorf("linker error: zero page overflow: %d bytes allocated exceeds %d bytes limit ($0000-$00FF)",
			currentZPUsage, ZPMaxSize)
	}
	if currentRAMUsage > RAMMaxSize {
		return fmt.Errorf("linker error: main RAM overflow: %d bytes allocated exceeds %d bytes limit ($0300-$07FF)",
			currentRAMUsage, RAMMaxSize)
	}
	if currentWRAMUsage > WRAMMaxSize {
		return fmt.Errorf("linker error: work RAM overflow: %d bytes allocated exceeds %d bytes limit ($6000-$7FFF)",
			currentWRAMUsage, WRAMMaxSize)
	}
	if currentWRAMUsage > 0 {
		l.config.HasBattery = true
	}

	for fileIdx, objFile := range l.objects {
		for _, sym := range objFile.Symbols {
			if sym.Type == obj.SymbolTypeImport || strings.HasPrefix(sym.Name, "__seg_") {
				// Imports are checked during relocation resolution; metadata symbols skipped
				continue
			}

			var finalOffset uint32
			var addr int64

			switch sym.Bank {
			case obj.BankConst:
				addr = sym.Value
			case obj.BankZP:
				finalOffset = fileZPBase[fileIdx] + uint32(sym.Value)
				addr = int64(ZPBaseAddr + finalOffset)
			case obj.BankRAM:
				finalOffset = fileRAMBase[fileIdx] + uint32(sym.Value)
				addr = int64(RAMBaseAddr + finalOffset)
			case obj.BankWRAM:
				finalOffset = fileWRAMBase[fileIdx] + uint32(sym.Value)
				addr = int64(WRAMBaseAddr + finalOffset)
			default:
				if sym.Bank >= 0 {
					chunkOffset := l.getChunkOffset(fileIdx, uint32(sym.Bank))
					finalOffset = chunkOffset + uint32(sym.Value)
					baseAddr := BankBaseAddress(uint32(sym.Bank))
					addr = baseAddr + int64(finalOffset)
				} else {
					addr = sym.Value
				}
			}

			resolved := &resolvedSymbol{
				Name:         sym.Name,
				Type:         sym.Type,
				Scope:        sym.Scope,
				Bank:         sym.Bank,
				OffsetInBank: finalOffset,
				Address:      addr,
				FileIndex:    fileIdx,
				SourceFile:   objFile.SourceFile,
			}

			if sym.Scope == obj.ScopeExport || sym.Scope == obj.ScopeGlobal {
				if existing, exists := l.globalSyms[sym.Name]; exists {
					return fmt.Errorf("linker error: duplicate symbol %q defined in %s and %s",
						sym.Name, existing.SourceFile, objFile.SourceFile)
				}
				l.globalSyms[sym.Name] = resolved
			}

			// Also store in local symbols keyed by file
			l.localSyms[symbolKey{fileIdx: fileIdx, name: sym.Name}] = resolved
		}
	}
	return nil
}

func (l *Linker) lookupSymbol(fileIdx int, name string) (*resolvedSymbol, bool) {
	// First check local/file-scoped symbols
	if sym, ok := l.localSyms[symbolKey{fileIdx: fileIdx, name: name}]; ok {
		return sym, true
	}
	// Next check global/exported symbols
	if sym, ok := l.globalSyms[name]; ok {
		return sym, true
	}
	return nil, false
}

func (l *Linker) applyRelocations() error {
	for _, placement := range l.placements {
		bIdx := placement.bankIndex
		fileIdx := placement.fileIndex
		chunk := placement.chunk
		startOffset := placement.startOffset

		for _, rel := range chunk.Relocations {
			targetSym, found := l.lookupSymbol(fileIdx, rel.Symbol)
			if !found {
				return fmt.Errorf("linker error: undefined symbol %q referenced in %s (bank %d, chunk offset $%04X)",
					rel.Symbol, l.objects[fileIdx].SourceFile, bIdx, rel.Offset)
			}

			relocOffset := startOffset + rel.Offset
			if relocOffset >= BankSize {
				return fmt.Errorf("linker error: relocation offset $%04X out of bounds in bank %d", relocOffset, bIdx)
			}

			targetAddr := targetSym.Address + rel.Addend

			switch rel.Type {
			case obj.RelocAddr16:
				if relocOffset+1 >= BankSize {
					return fmt.Errorf("linker error: 16-bit relocation offset $%04X out of bounds in bank %d", relocOffset, bIdx)
				}
				l.banks[bIdx][relocOffset] = byte(targetAddr & 0xFF)
				l.banks[bIdx][relocOffset+1] = byte((targetAddr >> 8) & 0xFF)

			case obj.RelocLowByte:
				l.banks[bIdx][relocOffset] = byte(targetAddr & 0xFF)

			case obj.RelocHighByte:
				l.banks[bIdx][relocOffset] = byte((targetAddr >> 8) & 0xFF)

			case obj.RelocBankByte:
				var bankByte byte
				if targetSym.Bank >= 0 {
					bankByte = byte(targetSym.Bank & 0xFF)
				} else {
					bankByte = byte((targetAddr >> 16) & 0xFF)
				}
				l.banks[bIdx][relocOffset] = bankByte

			case obj.RelocZeroPage8:
				if targetAddr < 0 || targetAddr > 0xFF {
					// Low byte allowed
				}
				l.banks[bIdx][relocOffset] = byte(targetAddr & 0xFF)

			case obj.RelocRelative8:
				// PC after the branch instruction
				bankBase := BankBaseAddress(bIdx)
				pcAfter := bankBase + int64(relocOffset) + 1
				disp := targetAddr - pcAfter
				if disp < -128 || disp > 127 {
					return fmt.Errorf("linker error: relative branch out of range (%d bytes) in %s to %q (bank %d, offset $%04X)",
						disp, l.objects[fileIdx].SourceFile, rel.Symbol, bIdx, relocOffset)
				}
				l.banks[bIdx][relocOffset] = byte(int8(disp))

			default:
				return fmt.Errorf("linker error: unsupported relocation type %v at offset $%04X in bank %d",
					rel.Type, relocOffset, bIdx)
			}
		}
	}
	return nil
}

func (l *Linker) setupVectors() error {
	bank63 := l.banks[NumBanks-1]

	// Find Reset Vector candidate
	resetCandidates := []string{"reset_handler", "reset", "start", "main"}
	var resetSym *resolvedSymbol
	for _, name := range resetCandidates {
		if sym, ok := l.globalSyms[name]; ok {
			resetSym = sym
			break
		}
	}

	// Find NMI Vector candidate
	nmiCandidates := []string{"nmi_handler", "nmi", "vblank_handler", "vblank"}
	var nmiSym *resolvedSymbol
	for _, name := range nmiCandidates {
		if sym, ok := l.globalSyms[name]; ok {
			nmiSym = sym
			break
		}
	}

	// Find IRQ Vector candidate
	irqCandidates := []string{"irq_handler", "irq"}
	var irqSym *resolvedSymbol
	for _, name := range irqCandidates {
		if sym, ok := l.globalSyms[name]; ok {
			irqSym = sym
			break
		}
	}

	// Populate NMI Vector at $FFFA ($1FFA in bank 63) if currently 0
	if bank63[NMIVectorOffset] == 0 && bank63[NMIVectorOffset+1] == 0 {
		var nmiAddr uint16
		if nmiSym != nil {
			nmiAddr = uint16(nmiSym.Address)
		} else if resetSym != nil {
			nmiAddr = uint16(resetSym.Address)
		}
		binary.LittleEndian.PutUint16(bank63[NMIVectorOffset:NMIVectorOffset+2], nmiAddr)
	}

	// Populate Reset Vector at $FFFC ($1FFC in bank 63) if currently 0
	if bank63[ResetVectorOffset] == 0 && bank63[ResetVectorOffset+1] == 0 {
		if resetSym == nil {
			return fmt.Errorf("linker error: reset vector not found (expected one of %v)", resetCandidates)
		}
		resetAddr := uint16(resetSym.Address)
		binary.LittleEndian.PutUint16(bank63[ResetVectorOffset:ResetVectorOffset+2], resetAddr)
	}

	// Populate IRQ Vector at $FFFE ($1FFE in bank 63) if currently 0
	if bank63[IRQVectorOffset] == 0 && bank63[IRQVectorOffset+1] == 0 {
		var irqAddr uint16
		if irqSym != nil {
			irqAddr = uint16(irqSym.Address)
		} else if resetSym != nil {
			irqAddr = uint16(resetSym.Address)
		}
		binary.LittleEndian.PutUint16(bank63[IRQVectorOffset:IRQVectorOffset+2], irqAddr)
	}

	return nil
}

func (l *Linker) buildINESImage() []byte {
	out := make([]byte, TotalOutputSize)

	// 16-byte iNES Header
	out[0] = 'N'
	out[1] = 'E'
	out[2] = 'S'
	out[3] = 0x1A

	// PRG ROM size in 16KB units: 512KB / 16KB = 32
	out[4] = byte(NumBanks / 2) // 32 (0x20)

	// CHR ROM size in 8KB units: 0 for 8KB CHR-RAM
	out[5] = 0

	// Flags 6: Mapper lower nibble (MMC3 is mapper 4 -> 0x40) | Mirroring
	flags6 := (l.config.Mapper & 0x0F) << 4
	if l.config.Mirroring != 0 {
		flags6 |= 0x01
	}
	if l.config.HasBattery {
		flags6 |= 0x02
	}
	out[6] = flags6

	// Flags 7: Mapper upper nibble
	flags7 := l.config.Mapper & 0xF0
	out[7] = flags7

	// Flags 8: PRG-RAM size (1 x 8KB unit)
	out[8] = 0x01

	// Flags 9: TV system (0: NTSC)
	out[9] = 0x00

	// Flags 10: Unused / TV system
	out[10] = 0x00

	// Bytes 11-15: Zero padding

	// Copy PRG ROM Banks
	offset := INESHeaderSize
	for i := 0; i < NumBanks; i++ {
		copy(out[offset:offset+BankSize], l.banks[i])
		offset += BankSize
	}

	return out
}

// LinkFiles is a helper to read multiple .mo files, link them, and write the .nes output file.
func LinkFiles(inputPaths []string, outputPath string) error {
	if len(inputPaths) == 0 {
		return fmt.Errorf("no input files provided")
	}

	objects := make([]*obj.ObjectFile, len(inputPaths))
	for i, path := range inputPaths {
		objFile, err := obj.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read object file %q: %w", path, err)
		}
		objects[i] = objFile
	}

	linker := NewLinker(objects...)
	romData, err := linker.Link()
	if err != nil {
		return err
	}

	if outputPath == "" {
		ext := filepath.Ext(inputPaths[0])
		if ext != "" {
			outputPath = strings.TrimSuffix(inputPaths[0], ext) + ".nes"
		} else {
			outputPath = inputPaths[0] + ".nes"
		}
	}

	if err := os.WriteFile(outputPath, romData, 0644); err != nil {
		return fmt.Errorf("failed to write NES ROM %q: %w", outputPath, err)
	}

	return nil
}
