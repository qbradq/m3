package asm

import (
	"fmt"
	"image"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/qbradq/m3/pkg/cpu6502"
	"github.com/qbradq/m3/pkg/gfx"
	"github.com/qbradq/m3/pkg/obj"
)

type Assembler struct {
	filename       string
	baseDir        string
	stmts          []Statement
	object         *obj.ObjectFile
	symbols        map[string]symbolEntry
	exports        map[string]bool
	imports        map[string]bool
	importZP       map[string]bool
	anonLabels     []anonLabelEntry
	currentSegment MemorySegment
	currentBank    uint32
	zpOffset       uint32
	ramOffset      uint32
	wramOffset     uint32
}

type symbolEntry struct {
	name  string
	sTyp  obj.SymbolType
	scope obj.SymbolScope
	bank  int32
	val   int64
}

type anonLabelEntry struct {
	anonIndex int
	stmtIndex int
	bank      uint32
	offset    uint32
}

func NewAssembler(filename string, stmts []Statement) *Assembler {
	baseDir := filepath.Dir(filename)
	return &Assembler{
		filename:       filename,
		baseDir:        baseDir,
		stmts:          stmts,
		object:         obj.NewObjectFile(filename),
		symbols:        make(map[string]symbolEntry),
		exports:        make(map[string]bool),
		imports:        make(map[string]bool),
		importZP:       make(map[string]bool),
		anonLabels:     make([]anonLabelEntry, 0),
		currentSegment: SegmentPRG,
		currentBank:    0,
		zpOffset:       0,
		ramOffset:      0,
		wramOffset:     0,
	}
}

func Assemble(filename string, src string) (*obj.ObjectFile, error) {
	lexer := NewLexer(filename, src)
	parser, err := NewParser(lexer)
	if err != nil {
		return nil, err
	}
	stmts, err := parser.ParseProgram()
	if err != nil {
		return nil, err
	}
	asm := NewAssembler(filename, stmts)
	return asm.Run()
}

func (a *Assembler) LookupSymbol(name string) (int64, int, bool) {
	sym, ok := a.symbols[name]
	if !ok {
		return 0, -1, false
	}
	return sym.val, int(sym.bank), true
}

func (a *Assembler) currentBankInt32() int32 {
	if a.currentBank == obj.BankAutoIndex {
		return obj.BankAuto
	}
	return int32(a.currentBank)
}

func (a *Assembler) Run() (*obj.ObjectFile, error) {
	// Pass 1: Handle symbols, labels, scopes, and calculate sizes/offsets
	if err := a.pass1(); err != nil {
		return nil, err
	}

	// Pass 2: Emit machine code, resolve symbols, record relocations
	if err := a.pass2(); err != nil {
		return nil, err
	}

	// Populate symbols in object file
	for _, sym := range a.symbols {
		scope := sym.scope
		if a.exports[sym.name] {
			scope = obj.ScopeExport
		}
		a.object.AddSymbol(obj.Symbol{
			Name:  sym.name,
			Type:  sym.sTyp,
			Scope: scope,
			Bank:  sym.bank,
			Value: sym.val,
		})
	}
	for imp := range a.imports {
		a.object.AddSymbol(obj.Symbol{
			Name:  imp,
			Type:  obj.SymbolTypeImport,
			Scope: obj.ScopeGlobal,
			Bank:  -1,
			Value: 0,
		})
	}

	// Record RAM segment allocation sizes
	if a.zpOffset > 0 {
		a.object.AddSymbol(obj.Symbol{
			Name:  "__seg_zp_size__",
			Type:  obj.SymbolTypeConst,
			Scope: obj.ScopeLocal,
			Bank:  obj.BankZP,
			Value: int64(a.zpOffset),
		})
	}
	if a.ramOffset > 0 {
		a.object.AddSymbol(obj.Symbol{
			Name:  "__seg_ram_size__",
			Type:  obj.SymbolTypeConst,
			Scope: obj.ScopeLocal,
			Bank:  obj.BankRAM,
			Value: int64(a.ramOffset),
		})
	}
	if a.wramOffset > 0 {
		a.object.AddSymbol(obj.Symbol{
			Name:  "__seg_wram_size__",
			Type:  obj.SymbolTypeConst,
			Scope: obj.ScopeLocal,
			Bank:  obj.BankWRAM,
			Value: int64(a.wramOffset),
		})
	}

	return a.object, nil
}

func (a *Assembler) pass1() error {
	a.currentSegment = SegmentPRG
	a.currentBank = 0
	a.zpOffset = 0
	a.ramOffset = 0
	a.wramOffset = 0
	bankOffsets := make(map[uint32]uint32)
	var currentGlobalLabel string
	var scopePrefix string
	var anonIndex int

	for i, stmt := range a.stmts {
		switch s := stmt.(type) {
		case *MemoryDirective:
			a.currentSegment = s.Segment
			if s.Size != nil {
				sizeVal, err := s.Size.Eval(a)
				if err != nil {
					return fmt.Errorf("%s: memory allocation size must evaluate to a constant in pass 1: %w", s.Pos(), err)
				}
				switch s.Segment {
				case SegmentZP:
					a.zpOffset += uint32(sizeVal)
				case SegmentRAM:
					a.ramOffset += uint32(sizeVal)
				case SegmentWRAM:
					a.wramOffset += uint32(sizeVal)
				}
			}

		case *BankDirective:
			a.currentSegment = SegmentPRG
			if s.IsAuto {
				a.currentBank = obj.BankAutoIndex
			} else {
				val, err := s.BankIndex.Eval(a)
				if err != nil {
					return fmt.Errorf("%s: bank index must be a constant expression in pass 1: %w", s.Pos(), err)
				}
				a.currentBank = uint32(val)
			}

		case *ExportDirective:
			for _, name := range s.Names {
				a.exports[name] = true
			}

		case *ImportDirective:
			for _, name := range s.Names {
				a.imports[name] = true
				if s.IsZeroPage {
					a.importZP[name] = true
				}
			}

		case *ScopeDirective:
			if s.IsEnd {
				scopePrefix = ""
			} else {
				scopePrefix = s.Name
				if s.IsProc && s.Name != "" {
					var currentOffset uint32
					var currentBank int32
					switch a.currentSegment {
					case SegmentPRG:
						currentOffset = bankOffsets[a.currentBank]
						if a.currentBank == obj.BankAutoIndex {
							currentBank = obj.BankAuto
						} else {
							currentBank = int32(a.currentBank)
						}
					case SegmentZP:
						currentOffset = a.zpOffset
						currentBank = obj.BankZP
					case SegmentRAM:
						currentOffset = a.ramOffset
						currentBank = obj.BankRAM
					case SegmentWRAM:
						currentOffset = a.wramOffset
						currentBank = obj.BankWRAM
					}
					currentGlobalLabel = s.Name
					a.symbols[s.Name] = symbolEntry{
						name:  s.Name,
						sTyp:  obj.SymbolTypeLabel,
						scope: obj.ScopeGlobal,
						bank:  currentBank,
						val:   int64(currentOffset),
					}
				}
			}

		case *AssignmentStmt:
			val, err := s.Value.Eval(a)
			fullName := s.Name
			if scopePrefix != "" && !strings.Contains(s.Name, "::") {
				fullName = scopePrefix + "::" + s.Name
			}
			if err == nil {
				a.symbols[fullName] = symbolEntry{
					name:  fullName,
					sTyp:  obj.SymbolTypeConst,
					scope: obj.ScopeLocal,
					bank:  obj.BankConst,
					val:   val,
				}
				if fullName != s.Name {
					a.symbols[s.Name] = symbolEntry{
						name:  s.Name,
						sTyp:  obj.SymbolTypeConst,
						scope: obj.ScopeLocal,
						bank:  obj.BankConst,
						val:   val,
					}
				}
			}

		case *DefineDirective:
			val, err := s.Value.Eval(a)
			fullName := s.Name
			if scopePrefix != "" && !strings.Contains(s.Name, "::") {
				fullName = scopePrefix + "::" + s.Name
			}
			if err == nil {
				a.symbols[fullName] = symbolEntry{
					name:  fullName,
					sTyp:  obj.SymbolTypeConst,
					scope: obj.ScopeLocal,
					bank:  obj.BankConst,
					val:   val,
				}
				if fullName != s.Name {
					a.symbols[s.Name] = symbolEntry{
						name:  s.Name,
						sTyp:  obj.SymbolTypeConst,
						scope: obj.ScopeLocal,
						bank:  obj.BankConst,
						val:   val,
					}
				}
			}
			if s.IsExport {
				a.exports[s.Name] = true
				a.exports[fullName] = true
			}

		case *LabelStmt:
			var currentOffset uint32
			var currentBank int32
			switch a.currentSegment {
			case SegmentPRG:
				currentOffset = bankOffsets[a.currentBank]
				if a.currentBank == obj.BankAutoIndex {
					currentBank = obj.BankAuto
				} else {
					currentBank = int32(a.currentBank)
				}
			case SegmentZP:
				currentOffset = a.zpOffset
				currentBank = obj.BankZP
			case SegmentRAM:
				currentOffset = a.ramOffset
				currentBank = obj.BankRAM
			case SegmentWRAM:
				currentOffset = a.wramOffset
				currentBank = obj.BankWRAM
			}

			if s.IsAnon {
				anonName := fmt.Sprintf("__anon_%d", anonIndex)
				a.anonLabels = append(a.anonLabels, anonLabelEntry{
					anonIndex: anonIndex,
					stmtIndex: i,
					bank:      a.currentBank,
					offset:    currentOffset,
				})
				a.symbols[anonName] = symbolEntry{
					name:  anonName,
					sTyp:  obj.SymbolTypeLabel,
					scope: obj.ScopeLocal,
					bank:  currentBank,
					val:   int64(currentOffset),
				}
				anonIndex++
			} else if s.IsLocal {
				fullName := currentGlobalLabel + s.Name
				a.symbols[fullName] = symbolEntry{
					name:  fullName,
					sTyp:  obj.SymbolTypeLabel,
					scope: obj.ScopeLocal,
					bank:  currentBank,
					val:   int64(currentOffset),
				}
			} else {
				fullName := s.Name
				if scopePrefix != "" {
					fullName = scopePrefix + "::" + s.Name
				}
				currentGlobalLabel = fullName
				a.symbols[fullName] = symbolEntry{
					name:  fullName,
					sTyp:  obj.SymbolTypeLabel,
					scope: obj.ScopeGlobal,
					bank:  currentBank,
					val:   int64(currentOffset),
				}
			}

		case *InstructionStmt:
			size := a.calculateInstructionSize(s)
			bankOffsets[a.currentBank] += uint32(size)

		case *DataDirective:
			size, err := a.calculateDataSize(s)
			if err != nil {
				return err
			}
			bankOffsets[a.currentBank] += uint32(size)

		case *ReserveDirective:
			sizeVal, err := s.Size.Eval(a)
			if err != nil {
				return fmt.Errorf("%s: .res size must evaluate to a constant in pass 1: %w", s.Pos(), err)
			}
			switch a.currentSegment {
			case SegmentPRG:
				bankOffsets[a.currentBank] += uint32(sizeVal)
			case SegmentZP:
				a.zpOffset += uint32(sizeVal)
			case SegmentRAM:
				a.ramOffset += uint32(sizeVal)
			case SegmentWRAM:
				a.wramOffset += uint32(sizeVal)
			}

		case *IncludeDirective:
			if s.IsBin {
				size, err := a.getIncbinSize(s)
				if err != nil {
					return err
				}
				bankOffsets[a.currentBank] += uint32(size)
			}

		case *IncchrDirective:
			size, err := a.getIncchrSize(s)
			if err != nil {
				return err
			}
			bankOffsets[a.currentBank] += uint32(size)

		case *IncpalDirective:
			size, err := a.getIncpalSize(s)
			if err != nil {
				return err
			}
			bankOffsets[a.currentBank] += uint32(size)
		}
	}
	return nil
}

func (a *Assembler) calculateInstructionSize(inst *InstructionStmt) int {
	switch inst.Mode {
	case cpu6502.ModeImplied, cpu6502.ModeAccumulator:
		return 1
	case cpu6502.ModeImmediate, cpu6502.ModeZeroPage, cpu6502.ModeZeroPageX, cpu6502.ModeZeroPageY,
		cpu6502.ModeIndirectX, cpu6502.ModeIndirectY, cpu6502.ModeRelative:
		return 2
	case cpu6502.ModeAbsolute, cpu6502.ModeAbsoluteX, cpu6502.ModeAbsoluteY, cpu6502.ModeIndirect:
		// Check if operand is small and can be ZeroPage if instruction supports it
		if inst.Operand != nil {
			if u, ok := inst.Operand.(*UnaryExpr); ok && u.Op == UnaryZeroPage {
				if inst.Mode == cpu6502.ModeAbsolute {
					inst.Mode = cpu6502.ModeZeroPage
					return 2
				} else if inst.Mode == cpu6502.ModeAbsoluteX {
					inst.Mode = cpu6502.ModeZeroPageX
					return 2
				} else if inst.Mode == cpu6502.ModeAbsoluteY {
					inst.Mode = cpu6502.ModeZeroPageY
					return 2
				}
			}
			if u, ok := inst.Operand.(*UnaryExpr); ok && u.Op == UnaryAbsolute {
				return 3
			}
			// Check if symbol is in ZP
			symName := a.symbolName(inst.Operand, "")
			if (symName != "" && a.symbols[symName].bank == obj.BankZP) || a.importZP[symName] {
				if inst.Mode == cpu6502.ModeAbsolute {
					if _, ok := cpu6502.LookupInstruction(inst.Mnemonic, cpu6502.ModeZeroPage); ok {
						inst.Mode = cpu6502.ModeZeroPage
						return 2
					}
				} else if inst.Mode == cpu6502.ModeAbsoluteX {
					if _, ok := cpu6502.LookupInstruction(inst.Mnemonic, cpu6502.ModeZeroPageX); ok {
						inst.Mode = cpu6502.ModeZeroPageX
						return 2
					}
				} else if inst.Mode == cpu6502.ModeAbsoluteY {
					if _, ok := cpu6502.LookupInstruction(inst.Mnemonic, cpu6502.ModeZeroPageY); ok {
						inst.Mode = cpu6502.ModeZeroPageY
						return 2
					}
				}
			}

			// If constant <= 0xFF and not absolute forced, optimize to ZP
			if val, isConst := a.isConstantExpr(inst.Operand, ""); isConst && val >= 0 && val <= 0xFF {
				if inst.Mode == cpu6502.ModeAbsolute {
					if _, ok := cpu6502.LookupInstruction(inst.Mnemonic, cpu6502.ModeZeroPage); ok {
						inst.Mode = cpu6502.ModeZeroPage
						return 2
					}
				} else if inst.Mode == cpu6502.ModeAbsoluteX {
					if _, ok := cpu6502.LookupInstruction(inst.Mnemonic, cpu6502.ModeZeroPageX); ok {
						inst.Mode = cpu6502.ModeZeroPageX
						return 2
					}
				} else if inst.Mode == cpu6502.ModeAbsoluteY {
					if _, ok := cpu6502.LookupInstruction(inst.Mnemonic, cpu6502.ModeZeroPageY); ok {
						inst.Mode = cpu6502.ModeZeroPageY
						return 2
					}
				}
			}
		}
		return 3
	default:
		return 3
	}
}

func (a *Assembler) calculateDataSize(d *DataDirective) (int, error) {
	total := 0
	for _, item := range d.Items {
		if item.IsStr {
			if d.Type == DataAsciiz {
				total += len(item.String) + 1
			} else {
				total += len(item.String)
			}
		} else {
			switch d.Type {
			case DataByte:
				total += 1
			case DataWord:
				total += 2
			case DataDword:
				total += 4
			case DataAsciiz:
				total += 1
			}
		}
	}
	return total, nil
}

func (a *Assembler) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if a.baseDir != "" {
		p := filepath.Join(a.baseDir, path)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if a.baseDir != "" {
		return filepath.Join(a.baseDir, path)
	}
	return path
}

func (a *Assembler) getIncbinSize(inc *IncludeDirective) (int, error) {
	path := a.resolvePath(inc.Filename)
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("%s: cannot stat incbin file %q: %w", inc.Pos(), path, err)
	}
	fullSize := int(info.Size())
	offset := 0
	if inc.Offset != nil {
		oVal, err := inc.Offset.Eval(a)
		if err != nil {
			return 0, err
		}
		offset = int(oVal)
	}
	if inc.Size != nil {
		sVal, err := inc.Size.Eval(a)
		if err != nil {
			return 0, err
		}
		return int(sVal), nil
	}
	return fullSize - offset, nil
}

func (a *Assembler) pass2() error {
	a.currentSegment = SegmentPRG
	a.currentBank = 0
	var currentGlobalLabel string
	var scopePrefix string

	for i, stmt := range a.stmts {
		switch s := stmt.(type) {
		case *MemoryDirective:
			a.currentSegment = s.Segment

		case *BankDirective:
			a.currentSegment = SegmentPRG
			if s.IsAuto {
				a.currentBank = obj.BankAutoIndex
			} else {
				val, _ := s.BankIndex.Eval(a)
				a.currentBank = uint32(val)
			}

		case *ScopeDirective:
			if s.IsEnd {
				scopePrefix = ""
			} else {
				scopePrefix = s.Name
				if s.IsProc && s.Name != "" {
					currentGlobalLabel = s.Name
				}
			}

		case *LabelStmt:
			if !s.IsAnon && !s.IsLocal {
				fullName := s.Name
				if scopePrefix != "" {
					fullName = scopePrefix + "::" + s.Name
				}
				currentGlobalLabel = fullName
			}

		case *AssignmentStmt:
			val, err := s.Value.Eval(a)
			fullName := s.Name
			if scopePrefix != "" && !strings.Contains(s.Name, "::") {
				fullName = scopePrefix + "::" + s.Name
			}
			if err == nil {
				a.symbols[fullName] = symbolEntry{
					name:  fullName,
					sTyp:  obj.SymbolTypeConst,
					scope: obj.ScopeLocal,
					bank:  obj.BankConst,
					val:   val,
				}
				if fullName != s.Name {
					a.symbols[s.Name] = symbolEntry{
						name:  s.Name,
						sTyp:  obj.SymbolTypeConst,
						scope: obj.ScopeLocal,
						bank:  obj.BankConst,
						val:   val,
					}
				}
			}

		case *DefineDirective:
			val, err := s.Value.Eval(a)
			if err != nil {
				return fmt.Errorf("%s: cannot evaluate constant expression for .define %q: %w", s.Pos(), s.Name, err)
			}
			fullName := s.Name
			if scopePrefix != "" && !strings.Contains(s.Name, "::") {
				fullName = scopePrefix + "::" + s.Name
			}
			a.symbols[fullName] = symbolEntry{
				name:  fullName,
				sTyp:  obj.SymbolTypeConst,
				scope: obj.ScopeLocal,
				bank:  obj.BankConst,
				val:   val,
			}
			if fullName != s.Name {
				a.symbols[s.Name] = symbolEntry{
					name:  s.Name,
					sTyp:  obj.SymbolTypeConst,
					scope: obj.ScopeLocal,
					bank:  obj.BankConst,
					val:   val,
				}
			}
			if s.IsExport {
				a.exports[s.Name] = true
				a.exports[fullName] = true
			}

		case *InstructionStmt:
			if err := a.emitInstruction(s, i, currentGlobalLabel); err != nil {
				return err
			}

		case *DataDirective:
			if err := a.emitData(s, currentGlobalLabel); err != nil {
				return err
			}

		case *ReserveDirective:
			if err := a.emitReserve(s); err != nil {
				return err
			}

		case *IncludeDirective:
			if s.IsBin {
				if err := a.emitIncbin(s); err != nil {
					return err
				}
			}

		case *IncchrDirective:
			if err := a.emitIncchr(s); err != nil {
				return err
			}

		case *IncpalDirective:
			if err := a.emitIncpal(s); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Assembler) emitInstruction(inst *InstructionStmt, stmtIdx int, currentGlobal string) error {
	bank := a.object.GetOrCreateBank(a.currentBank)
	currentOffset := uint32(len(bank.Data))

	info, ok := cpu6502.LookupInstruction(inst.Mnemonic, inst.Mode)
	if !ok {
		return fmt.Errorf("%s: invalid addressing mode %s for mnemonic %s", inst.Pos(), inst.Mode, inst.Mnemonic)
	}

	// Emit opcode
	bank.Data = append(bank.Data, info.Opcode)

	// If operand is an anonymous label reference :+, :-, etc.
	if symExpr, ok := inst.Operand.(*SymbolExpr); ok && strings.HasPrefix(symExpr.Name, ":") {
		if inst.Mode == cpu6502.ModeRelative {
			targetOffset, targetBank, _, err := a.resolveAnonLabel(symExpr.Name, stmtIdx)
			if err != nil {
				return fmt.Errorf("%s: %w", inst.Pos(), err)
			}
			if targetBank != a.currentBank {
				return fmt.Errorf("%s: relative branch across banks (from bank %d to bank %d)", inst.Pos(), a.currentBank, targetBank)
			}
			pcAfter := currentOffset + 2
			disp := int64(targetOffset) - int64(pcAfter)
			if disp < -128 || disp > 127 {
				return fmt.Errorf("%s: relative branch out of range (%d bytes)", inst.Pos(), disp)
			}
			bank.Data = append(bank.Data, byte(int8(disp)))
			return nil
		} else {
			_, _, anonSymName, err := a.resolveAnonLabel(symExpr.Name, stmtIdx)
			if err != nil {
				return fmt.Errorf("%s: %w", inst.Pos(), err)
			}
			inst.Operand = &SymbolExpr{Name: anonSymName, pos: symExpr.pos}
		}
	}

	// Emit operands
	switch inst.Mode {
	case cpu6502.ModeImplied, cpu6502.ModeAccumulator:
		return nil

	case cpu6502.ModeImmediate:
		relocType, symName, addend, isReloc := a.extractReloc(inst.Operand, currentGlobal, obj.RelocLowByte)
		if isReloc {
			bank.Relocations = append(bank.Relocations, obj.Relocation{
				Offset: currentOffset + 1,
				Symbol: symName,
				Type:   relocType,
				Addend: addend,
			})
			bank.Data = append(bank.Data, 0x00)
			return nil
		}
		val, err := inst.Operand.Eval(a)
		if err != nil {
			return fmt.Errorf("%s: cannot evaluate immediate operand: %w", inst.Pos(), err)
		}
		bank.Data = append(bank.Data, byte(val&0xFF))
		return nil

	case cpu6502.ModeZeroPage, cpu6502.ModeZeroPageX, cpu6502.ModeZeroPageY,
		cpu6502.ModeIndirectX, cpu6502.ModeIndirectY:
		relocType, symName, addend, isReloc := a.extractReloc(inst.Operand, currentGlobal, obj.RelocZeroPage8)
		if isReloc {
			bank.Relocations = append(bank.Relocations, obj.Relocation{
				Offset: currentOffset + 1,
				Symbol: symName,
				Type:   relocType,
				Addend: addend,
			})
			bank.Data = append(bank.Data, 0x00)
			return nil
		}
		val, err := inst.Operand.Eval(a)
		if err != nil {
			return fmt.Errorf("%s: cannot evaluate zero-page operand: %w", inst.Pos(), err)
		}
		bank.Data = append(bank.Data, byte(val&0xFF))
		return nil

	case cpu6502.ModeAbsolute, cpu6502.ModeAbsoluteX, cpu6502.ModeAbsoluteY, cpu6502.ModeIndirect:
		relocType, symName, addend, isReloc := a.extractReloc(inst.Operand, currentGlobal, obj.RelocAddr16)
		if isReloc {
			bank.Relocations = append(bank.Relocations, obj.Relocation{
				Offset: currentOffset + 1,
				Symbol: symName,
				Type:   relocType,
				Addend: addend,
			})
			bank.Data = append(bank.Data, 0x00, 0x00)
			return nil
		}
		val, err := inst.Operand.Eval(a)
		if err != nil {
			return fmt.Errorf("%s: cannot evaluate absolute operand: %w", inst.Pos(), err)
		}
		bank.Data = append(bank.Data, byte(val&0xFF), byte((val>>8)&0xFF))
		return nil

	case cpu6502.ModeRelative:
		// Check if local or global label defined in this chunk
		symName := a.symbolName(inst.Operand, currentGlobal)
		if sym, ok := a.symbols[symName]; ok && sym.sTyp == obj.SymbolTypeLabel && sym.bank == a.currentBankInt32() {
			pcAfter := currentOffset + 2
			disp := sym.val - int64(pcAfter)
			if disp < -128 || disp > 127 {
				return fmt.Errorf("%s: relative branch out of range (%d bytes)", inst.Pos(), disp)
			}
			bank.Data = append(bank.Data, byte(int8(disp)))
			return nil
		}

		// Otherwise emit relocation
		relocType, rSymName, addend, isReloc := a.extractReloc(inst.Operand, currentGlobal, obj.RelocRelative8)
		if isReloc {
			bank.Relocations = append(bank.Relocations, obj.Relocation{
				Offset: currentOffset + 1,
				Symbol: rSymName,
				Type:   relocType,
				Addend: addend,
			})
			bank.Data = append(bank.Data, 0x00)
			return nil
		}

		targetVal, err := inst.Operand.Eval(a)
		if err != nil {
			return fmt.Errorf("%s: cannot evaluate relative branch target: %w", inst.Pos(), err)
		}
		pcAfter := currentOffset + 2
		disp := targetVal - int64(pcAfter)
		if disp < -128 || disp > 127 {
			return fmt.Errorf("%s: relative branch out of range (%d bytes)", inst.Pos(), disp)
		}
		bank.Data = append(bank.Data, byte(int8(disp)))
		return nil
	}

	return nil
}

func (a *Assembler) emitData(d *DataDirective, currentGlobal string) error {
	bank := a.object.GetOrCreateBank(a.currentBank)

	for _, item := range d.Items {
		if item.IsStr {
			for i := 0; i < len(item.String); i++ {
				bank.Data = append(bank.Data, item.String[i])
			}
			if d.Type == DataAsciiz {
				bank.Data = append(bank.Data, 0x00)
			}
		} else {
			currentOffset := uint32(len(bank.Data))
			switch d.Type {
			case DataByte:
				relocType, symName, addend, isReloc := a.extractReloc(item.Expr, currentGlobal, obj.RelocLowByte)
				if isReloc {
					bank.Relocations = append(bank.Relocations, obj.Relocation{
						Offset: currentOffset,
						Symbol: symName,
						Type:   relocType,
						Addend: addend,
					})
					bank.Data = append(bank.Data, 0x00)
				} else {
					val, err := item.Expr.Eval(a)
					if err != nil {
						return fmt.Errorf("%s: cannot evaluate .byte expression: %w", item.Expr.Pos(), err)
					}
					bank.Data = append(bank.Data, byte(val&0xFF))
				}

			case DataWord:
				relocType, symName, addend, isReloc := a.extractReloc(item.Expr, currentGlobal, obj.RelocAddr16)
				if isReloc {
					bank.Relocations = append(bank.Relocations, obj.Relocation{
						Offset: currentOffset,
						Symbol: symName,
						Type:   relocType,
						Addend: addend,
					})
					bank.Data = append(bank.Data, 0x00, 0x00)
				} else {
					val, err := item.Expr.Eval(a)
					if err != nil {
						return fmt.Errorf("%s: cannot evaluate .word expression: %w", item.Expr.Pos(), err)
					}
					bank.Data = append(bank.Data, byte(val&0xFF), byte((val>>8)&0xFF))
				}

			case DataDword:
				val, err := item.Expr.Eval(a)
				if err != nil {
					return fmt.Errorf("%s: cannot evaluate .dword expression: %w", item.Expr.Pos(), err)
				}
				bank.Data = append(bank.Data,
					byte(val&0xFF),
					byte((val>>8)&0xFF),
					byte((val>>16)&0xFF),
					byte((val>>24)&0xFF),
				)

			case DataAsciiz:
				val, err := item.Expr.Eval(a)
				if err != nil {
					return fmt.Errorf("%s: cannot evaluate .asciiz expression: %w", item.Expr.Pos(), err)
				}
				bank.Data = append(bank.Data, byte(val&0xFF), 0x00)
			}
		}
	}
	return nil
}

func (a *Assembler) emitReserve(r *ReserveDirective) error {
	if a.currentSegment != SegmentPRG {
		return nil
	}
	bank := a.object.GetOrCreateBank(a.currentBank)
	sizeVal, err := r.Size.Eval(a)
	if err != nil {
		return fmt.Errorf("%s: cannot evaluate .res size: %w", r.Pos(), err)
	}
	fillByte := byte(0x00)
	if r.Fill != nil {
		fVal, err := r.Fill.Eval(a)
		if err != nil {
			return fmt.Errorf("%s: cannot evaluate .res fill: %w", r.Pos(), err)
		}
		fillByte = byte(fVal & 0xFF)
	}
	for i := int64(0); i < sizeVal; i++ {
		bank.Data = append(bank.Data, fillByte)
	}
	return nil
}

func (a *Assembler) emitIncbin(inc *IncludeDirective) error {
	bank := a.object.GetOrCreateBank(a.currentBank)
	path := a.resolvePath(inc.Filename)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s: failed to open incbin file %q: %w", inc.Pos(), path, err)
	}
	defer f.Close()

	offset := int64(0)
	if inc.Offset != nil {
		oVal, err := inc.Offset.Eval(a)
		if err != nil {
			return err
		}
		offset = oVal
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return fmt.Errorf("%s: failed to seek in incbin file %q: %w", inc.Pos(), path, err)
	}

	var data []byte
	if inc.Size != nil {
		sVal, err := inc.Size.Eval(a)
		if err != nil {
			return err
		}
		data = make([]byte, sVal)
		if _, err := io.ReadFull(f, data); err != nil {
			return fmt.Errorf("%s: failed to read %d bytes from incbin %q: %w", inc.Pos(), sVal, path, err)
		}
	} else {
		data, err = io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("%s: failed to read incbin %q: %w", inc.Pos(), path, err)
		}
	}

	bank.Data = append(bank.Data, data...)
	return nil
}

func (a *Assembler) getIncchrSize(inc *IncchrDirective) (int, error) {
	path := a.resolvePath(inc.Filename)
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("%s: cannot open PNG file %q: %w", inc.Pos(), path, err)
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, fmt.Errorf("%s: cannot decode PNG config for %q: %w", inc.Pos(), path, err)
	}
	if cfg.Width%8 != 0 || cfg.Height%8 != 0 {
		return 0, fmt.Errorf("%s: PNG dimensions (%dx%d) must be multiples of 8", inc.Pos(), cfg.Width, cfg.Height)
	}
	totalTiles := (cfg.Width / 8) * (cfg.Height / 8)
	return totalTiles * 16, nil
}

func (a *Assembler) getIncpalSize(inc *IncpalDirective) (int, error) {
	if inc.Count != nil {
		cVal, err := inc.Count.Eval(a)
		if err != nil {
			return 0, fmt.Errorf("%s: cannot evaluate palette count in pass 1: %w", inc.Pos(), err)
		}
		if cVal <= 0 {
			return 0, fmt.Errorf("%s: palette count must be positive, got %d", inc.Pos(), cVal)
		}
		return int(cVal), nil
	}
	return 4, nil
}

func (a *Assembler) emitIncchr(inc *IncchrDirective) error {
	bank := a.object.GetOrCreateBank(a.currentBank)
	path := a.resolvePath(inc.Filename)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s: failed to open PNG file %q: %w", inc.Pos(), path, err)
	}
	defer f.Close()

	chrBytes, err := gfx.ConvertPNGToCHR(f)
	if err != nil {
		return fmt.Errorf("%s: failed to convert PNG %q to CHR: %w", inc.Pos(), path, err)
	}
	bank.Data = append(bank.Data, chrBytes...)
	return nil
}

func (a *Assembler) emitIncpal(inc *IncpalDirective) error {
	bank := a.object.GetOrCreateBank(a.currentBank)
	path := a.resolvePath(inc.Filename)
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%s: failed to open PNG file %q: %w", inc.Pos(), path, err)
	}
	defer f.Close()

	count := 4
	if inc.Count != nil {
		cVal, err := inc.Count.Eval(a)
		if err != nil {
			return fmt.Errorf("%s: cannot evaluate palette count: %w", inc.Pos(), err)
		}
		count = int(cVal)
	}

	palBytes, err := gfx.ExtractPNGPalette(f, count)
	if err != nil {
		return fmt.Errorf("%s: failed to extract palette from PNG %q: %w", inc.Pos(), path, err)
	}
	bank.Data = append(bank.Data, palBytes...)
	return nil
}

func (a *Assembler) resolveAnonLabel(ref string, currentStmtIdx int) (uint32, uint32, string, error) {
	// ref is :+, :++, :-, :--
	isForward := strings.Contains(ref, "+")
	count := strings.Count(ref, "+")
	if !isForward {
		count = strings.Count(ref, "-")
	}

	if isForward {
		found := 0
		for _, anon := range a.anonLabels {
			if anon.stmtIndex > currentStmtIdx {
				found++
				if found == count {
					return anon.offset, anon.bank, fmt.Sprintf("__anon_%d", anon.anonIndex), nil
				}
			}
		}
		return 0, 0, "", fmt.Errorf("forward anonymous label %s not found", ref)
	} else {
		found := 0
		for i := len(a.anonLabels) - 1; i >= 0; i-- {
			anon := a.anonLabels[i]
			if anon.stmtIndex < currentStmtIdx {
				found++
				if found == count {
					return anon.offset, anon.bank, fmt.Sprintf("__anon_%d", anon.anonIndex), nil
				}
			}
		}
		return 0, 0, "", fmt.Errorf("backward anonymous label %s not found", ref)
	}
}

func (a *Assembler) isConstantExpr(expr Expr, currentGlobal string) (int64, bool) {
	if expr == nil {
		return 0, false
	}
	switch e := expr.(type) {
	case *NumberExpr:
		return e.Value, true
	case *SymbolExpr:
		name := e.Name
		if strings.HasPrefix(name, "@") {
			name = currentGlobal + name
		}
		sym, ok := a.symbols[name]
		if ok && sym.sTyp == obj.SymbolTypeConst {
			return sym.val, true
		}
		return 0, false
	case *UnaryExpr:
		val, ok := a.isConstantExpr(e.Right, currentGlobal)
		if !ok {
			return 0, false
		}
		switch e.Op {
		case UnaryPos:
			return val, true
		case UnaryNeg:
			return -val, true
		case UnaryBitNot:
			return ^val, true
		case UnaryLogNot:
			if val == 0 {
				return 1, true
			}
			return 0, true
		case UnaryLowByte:
			return val & 0xFF, true
		case UnaryHighByte:
			return (val >> 8) & 0xFF, true
		case UnaryBankByte:
			return (val >> 16) & 0xFF, true
		case UnaryZeroPage, UnaryAbsolute:
			return val, true
		}
		return 0, false
	case *BinaryExpr:
		// Check for label difference in same bank: label1 - label2
		if e.Op == BinSub {
			nameL := a.symbolName(e.Left, currentGlobal)
			nameR := a.symbolName(e.Right, currentGlobal)
			if nameL != "" && nameR != "" {
				sL, okL := a.symbols[nameL]
				sR, okR := a.symbols[nameR]
				if okL && okR && sL.sTyp == obj.SymbolTypeLabel && sR.sTyp == obj.SymbolTypeLabel && sL.bank == sR.bank && (sL.bank >= 0 || sL.bank == obj.BankAuto) {
					return sL.val - sR.val, true
				}
			}
		}
		lVal, lOk := a.isConstantExpr(e.Left, currentGlobal)
		rVal, rOk := a.isConstantExpr(e.Right, currentGlobal)
		if lOk && rOk {
			switch e.Op {
			case BinAdd:
				return lVal + rVal, true
			case BinSub:
				return lVal - rVal, true
			case BinMul:
				return lVal * rVal, true
			case BinDiv:
				if rVal != 0 {
					return lVal / rVal, true
				}
			case BinMod:
				if rVal != 0 {
					return lVal % rVal, true
				}
			case BinShl:
				return lVal << uint(rVal), true
			case BinShr:
				return int64(uint64(lVal) >> uint(rVal)), true
			case BinLt:
				if lVal < rVal {
					return 1, true
				}
				return 0, true
			case BinLtEq:
				if lVal <= rVal {
					return 1, true
				}
				return 0, true
			case BinGt:
				if lVal > rVal {
					return 1, true
				}
				return 0, true
			case BinGtEq:
				if lVal >= rVal {
					return 1, true
				}
				return 0, true
			case BinEq:
				if lVal == rVal {
					return 1, true
				}
				return 0, true
			case BinNotEq:
				if lVal != rVal {
					return 1, true
				}
				return 0, true
			case BinBitAnd:
				return lVal & rVal, true
			case BinBitXor:
				return lVal ^ rVal, true
			case BinBitOr:
				return lVal | rVal, true
			case BinLogAnd:
				if lVal != 0 && rVal != 0 {
					return 1, true
				}
				return 0, true
			case BinLogOr:
				if lVal != 0 || rVal != 0 {
					return 1, true
				}
				return 0, true
			}
		}
		return 0, false
	}
	return 0, false
}

func (a *Assembler) symbolName(expr Expr, currentGlobal string) string {
	if expr == nil {
		return ""
	}
	if s, ok := expr.(*SymbolExpr); ok {
		name := s.Name
		if strings.HasPrefix(name, "@") {
			name = currentGlobal + name
		}
		return name
	}
	if u, ok := expr.(*UnaryExpr); ok {
		return a.symbolName(u.Right, currentGlobal)
	}
	return ""
}

func (a *Assembler) extractReloc(expr Expr, currentGlobal string, defaultType obj.RelocType) (obj.RelocType, string, int64, bool) {
	if expr == nil {
		return defaultType, "", 0, false
	}

	// Check if already resolvable as a constant expression
	if _, isConst := a.isConstantExpr(expr, currentGlobal); isConst {
		return defaultType, "", 0, false
	}

	// Check unary operators: <, >, ^, z:, a:
	if u, ok := expr.(*UnaryExpr); ok {
		switch u.Op {
		case UnaryLowByte:
			return a.extractReloc(u.Right, currentGlobal, obj.RelocLowByte)
		case UnaryHighByte:
			return a.extractReloc(u.Right, currentGlobal, obj.RelocHighByte)
		case UnaryBankByte:
			return a.extractReloc(u.Right, currentGlobal, obj.RelocBankByte)
		case UnaryZeroPage:
			return a.extractReloc(u.Right, currentGlobal, obj.RelocZeroPage8)
		case UnaryAbsolute:
			return a.extractReloc(u.Right, currentGlobal, obj.RelocAddr16)
		default:
			return a.extractReloc(u.Right, currentGlobal, defaultType)
		}
	}

	// Check binary with addend: sym + const, const + sym, or sym - const
	if b, ok := expr.(*BinaryExpr); ok {
		if symName := a.symbolName(b.Left, currentGlobal); symName != "" {
			if val, isConst := a.isConstantExpr(b.Right, currentGlobal); isConst {
				addend := val
				if b.Op == BinSub {
					addend = -val
				}
				return defaultType, symName, addend, true
			}
		}
		if b.Op == BinAdd {
			if symName := a.symbolName(b.Right, currentGlobal); symName != "" {
				if val, isConst := a.isConstantExpr(b.Left, currentGlobal); isConst {
					return defaultType, symName, val, true
				}
			}
		}
	}

	// Direct symbol
	if symName := a.symbolName(expr, currentGlobal); symName != "" {
		return defaultType, symName, 0, true
	}

	return defaultType, "", 0, false
}
