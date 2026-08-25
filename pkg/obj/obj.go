package obj

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

var Magic = [6]byte{'M', '3', 'O', 'B', 'J', 1}

type SymbolType uint8

const (
	SymbolTypeLabel SymbolType = iota
	SymbolTypeConst
	SymbolTypeImport
)

func (t SymbolType) String() string {
	switch t {
	case SymbolTypeLabel:
		return "LABEL"
	case SymbolTypeConst:
		return "CONST"
	case SymbolTypeImport:
		return "IMPORT"
	default:
		return fmt.Sprintf("TYPE_%d", t)
	}
}

type SymbolScope uint8

const (
	ScopeLocal SymbolScope = iota
	ScopeGlobal
	ScopeExport
)

func (s SymbolScope) String() string {
	switch s {
	case ScopeLocal:
		return "LOCAL"
	case ScopeGlobal:
		return "GLOBAL"
	case ScopeExport:
		return "EXPORT"
	default:
		return fmt.Sprintf("SCOPE_%d", s)
	}
}

type RelocType uint8

const (
	RelocAddr16 RelocType = iota
	RelocLowByte
	RelocHighByte
	RelocBankByte
	RelocRelative8
	RelocZeroPage8
)

func (r RelocType) String() string {
	switch r {
	case RelocAddr16:
		return "ADDR16"
	case RelocLowByte:
		return "LOW_BYTE"
	case RelocHighByte:
		return "HIGH_BYTE"
	case RelocBankByte:
		return "BANK_BYTE"
	case RelocRelative8:
		return "REL8"
	case RelocZeroPage8:
		return "ZP8"
	default:
		return fmt.Sprintf("RELOC_%d", r)
	}
}

const (
	BankConst int32 = -1
	BankZP    int32 = -2
	BankRAM   int32 = -3
	BankWRAM  int32 = -4
	BankAuto  int32 = -5

	BankAutoIndex uint32 = 0xFFFFFFFF
)

type Symbol struct {
	Name  string
	Type  SymbolType
	Scope SymbolScope
	Bank  int32
	Value int64
}

type Relocation struct {
	Offset uint32
	Symbol string
	Type   RelocType
	Addend int64
}

type BankChunk struct {
	BankIndex   uint32
	Data        []byte
	Relocations []Relocation
}

type ObjectFile struct {
	SourceFile string
	Symbols    []Symbol
	Banks      []*BankChunk
}

func NewObjectFile(sourceFile string) *ObjectFile {
	return &ObjectFile{
		SourceFile: sourceFile,
		Symbols:    make([]Symbol, 0),
		Banks:      make([]*BankChunk, 0),
	}
}

func (o *ObjectFile) GetOrCreateBank(bankIndex uint32) *BankChunk {
	for _, b := range o.Banks {
		if b.BankIndex == bankIndex {
			return b
		}
	}
	b := &BankChunk{
		BankIndex:   bankIndex,
		Data:        make([]byte, 0),
		Relocations: make([]Relocation, 0),
	}
	o.Banks = append(o.Banks, b)
	return b
}

func (o *ObjectFile) AddSymbol(sym Symbol) {
	o.Symbols = append(o.Symbols, sym)
}

func (o *ObjectFile) Encode(w io.Writer) error {
	if _, err := w.Write(Magic[:]); err != nil {
		return fmt.Errorf("failed to write magic: %w", err)
	}

	if err := writeString(w, o.SourceFile); err != nil {
		return fmt.Errorf("failed to write source file name: %w", err)
	}

	// Write Symbols
	if err := binary.Write(w, binary.LittleEndian, uint32(len(o.Symbols))); err != nil {
		return err
	}
	for _, sym := range o.Symbols {
		if err := writeString(w, sym.Name); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint8(sym.Type)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint8(sym.Scope)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, sym.Bank); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, sym.Value); err != nil {
			return err
		}
	}

	// Write Banks
	if err := binary.Write(w, binary.LittleEndian, uint32(len(o.Banks))); err != nil {
		return err
	}
	for _, bank := range o.Banks {
		if err := binary.Write(w, binary.LittleEndian, bank.BankIndex); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(len(bank.Data))); err != nil {
			return err
		}
		if _, err := w.Write(bank.Data); err != nil {
			return err
		}

		if err := binary.Write(w, binary.LittleEndian, uint32(len(bank.Relocations))); err != nil {
			return err
		}
		for _, rel := range bank.Relocations {
			if err := binary.Write(w, binary.LittleEndian, rel.Offset); err != nil {
				return err
			}
			if err := writeString(w, rel.Symbol); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, uint8(rel.Type)); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, rel.Addend); err != nil {
				return err
			}
		}
	}

	return nil
}

func Decode(r io.Reader) (*ObjectFile, error) {
	var magic [6]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, fmt.Errorf("failed to read magic header: %w", err)
	}
	if magic != Magic {
		return nil, fmt.Errorf("invalid magic header or object version: %v", magic)
	}

	srcFile, err := readString(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file name: %w", err)
	}

	obj := NewObjectFile(srcFile)

	var numSymbols uint32
	if err := binary.Read(r, binary.LittleEndian, &numSymbols); err != nil {
		return nil, err
	}
	obj.Symbols = make([]Symbol, numSymbols)
	for i := uint32(0); i < numSymbols; i++ {
		name, err := readString(r)
		if err != nil {
			return nil, err
		}
		var symType, symScope uint8
		var bank int32
		var val int64
		if err := binary.Read(r, binary.LittleEndian, &symType); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &symScope); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &bank); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &val); err != nil {
			return nil, err
		}
		obj.Symbols[i] = Symbol{
			Name:  name,
			Type:  SymbolType(symType),
			Scope: SymbolScope(symScope),
			Bank:  bank,
			Value: val,
		}
	}

	var numBanks uint32
	if err := binary.Read(r, binary.LittleEndian, &numBanks); err != nil {
		return nil, err
	}
	obj.Banks = make([]*BankChunk, numBanks)
	for i := uint32(0); i < numBanks; i++ {
		var bankIndex uint32
		if err := binary.Read(r, binary.LittleEndian, &bankIndex); err != nil {
			return nil, err
		}
		var dataLen uint32
		if err := binary.Read(r, binary.LittleEndian, &dataLen); err != nil {
			return nil, err
		}
		data := make([]byte, dataLen)
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}

		var numRelocs uint32
		if err := binary.Read(r, binary.LittleEndian, &numRelocs); err != nil {
			return nil, err
		}
		relocs := make([]Relocation, numRelocs)
		for j := uint32(0); j < numRelocs; j++ {
			var offset uint32
			if err := binary.Read(r, binary.LittleEndian, &offset); err != nil {
				return nil, err
			}
			relSym, err := readString(r)
			if err != nil {
				return nil, err
			}
			var relType uint8
			if err := binary.Read(r, binary.LittleEndian, &relType); err != nil {
				return nil, err
			}
			var addend int64
			if err := binary.Read(r, binary.LittleEndian, &addend); err != nil {
				return nil, err
			}
			relocs[j] = Relocation{
				Offset: offset,
				Symbol: relSym,
				Type:   RelocType(relType),
				Addend: addend,
			}
		}

		obj.Banks[i] = &BankChunk{
			BankIndex:   bankIndex,
			Data:        data,
			Relocations: relocs,
		}
	}

	return obj, nil
}

func (o *ObjectFile) WriteFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return o.Encode(f)
}

func ReadFile(path string) (*ObjectFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Decode(f)
}

func writeString(w io.Writer, s string) error {
	data := []byte(s)
	if err := binary.Write(w, binary.LittleEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func readString(r io.Reader) (string, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func (o *ObjectFile) Dump() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Object File: %s\n", o.SourceFile)
	fmt.Fprintf(&buf, "Symbols (%d):\n", len(o.Symbols))
	for _, s := range o.Symbols {
		fmt.Fprintf(&buf, "  %-20s %-7s %-7s bank=%-3d val=$%04X (%d)\n", s.Name, s.Type, s.Scope, s.Bank, s.Value, s.Value)
	}
	fmt.Fprintf(&buf, "Banks (%d):\n", len(o.Banks))
	for _, b := range o.Banks {
		bankStr := fmt.Sprintf("%d", b.BankIndex)
		if b.BankIndex == BankAutoIndex {
			bankStr = "AUTO"
		}
		fmt.Fprintf(&buf, "  Bank %s: %d bytes, %d relocations\n", bankStr, len(b.Data), len(b.Relocations))
		for _, r := range b.Relocations {
			fmt.Fprintf(&buf, "    offset=$%04X type=%-10s sym=%s addend=%d\n", r.Offset, r.Type, r.Symbol, r.Addend)
		}
	}
	return buf.String()
}
