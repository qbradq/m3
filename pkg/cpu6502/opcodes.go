package cpu6502

import (
	"fmt"
	"strings"
)

type AddressingMode uint8

const (
	ModeImplied AddressingMode = iota
	ModeAccumulator
	ModeImmediate
	ModeZeroPage
	ModeZeroPageX
	ModeZeroPageY
	ModeAbsolute
	ModeAbsoluteX
	ModeAbsoluteY
	ModeIndirect
	ModeIndirectX
	ModeIndirectY
	ModeRelative
)

func (m AddressingMode) String() string {
	switch m {
	case ModeImplied:
		return "Implied"
	case ModeAccumulator:
		return "Accumulator"
	case ModeImmediate:
		return "Immediate"
	case ModeZeroPage:
		return "ZeroPage"
	case ModeZeroPageX:
		return "ZeroPageX"
	case ModeZeroPageY:
		return "ZeroPageY"
	case ModeAbsolute:
		return "Absolute"
	case ModeAbsoluteX:
		return "AbsoluteX"
	case ModeAbsoluteY:
		return "AbsoluteY"
	case ModeIndirect:
		return "Indirect"
	case ModeIndirectX:
		return "IndirectX"
	case ModeIndirectY:
		return "IndirectY"
	case ModeRelative:
		return "Relative"
	default:
		return fmt.Sprintf("Mode(%d)", m)
	}
}

type InstructionInfo struct {
	Mnemonic string
	Mode     AddressingMode
	Opcode   byte
	Size     int
}

type opcodeKey struct {
	mnemonic string
	mode     AddressingMode
}

var (
	instructions = []InstructionInfo{
		// ADC
		{"ADC", ModeImmediate, 0x69, 2},
		{"ADC", ModeZeroPage, 0x65, 2},
		{"ADC", ModeZeroPageX, 0x75, 2},
		{"ADC", ModeAbsolute, 0x6D, 3},
		{"ADC", ModeAbsoluteX, 0x7D, 3},
		{"ADC", ModeAbsoluteY, 0x79, 3},
		{"ADC", ModeIndirectX, 0x61, 2},
		{"ADC", ModeIndirectY, 0x71, 2},

		// AND
		{"AND", ModeImmediate, 0x29, 2},
		{"AND", ModeZeroPage, 0x25, 2},
		{"AND", ModeZeroPageX, 0x35, 2},
		{"AND", ModeAbsolute, 0x2D, 3},
		{"AND", ModeAbsoluteX, 0x3D, 3},
		{"AND", ModeAbsoluteY, 0x39, 3},
		{"AND", ModeIndirectX, 0x21, 2},
		{"AND", ModeIndirectY, 0x31, 2},

		// ASL
		{"ASL", ModeAccumulator, 0x0A, 1},
		{"ASL", ModeImplied, 0x0A, 1}, // ASL without operand defaults to accumulator
		{"ASL", ModeZeroPage, 0x06, 2},
		{"ASL", ModeZeroPageX, 0x16, 2},
		{"ASL", ModeAbsolute, 0x0E, 3},
		{"ASL", ModeAbsoluteX, 0x1E, 3},

		// BCC, BCS, BEQ, BMI, BNE, BPL, BVC, BVS
		{"BCC", ModeRelative, 0x90, 2},
		{"BCS", ModeRelative, 0xB0, 2},
		{"BEQ", ModeRelative, 0xF0, 2},
		{"BMI", ModeRelative, 0x30, 2},
		{"BNE", ModeRelative, 0xD0, 2},
		{"BPL", ModeRelative, 0x10, 2},
		{"BVC", ModeRelative, 0x50, 2},
		{"BVS", ModeRelative, 0x70, 2},

		// BIT
		{"BIT", ModeZeroPage, 0x24, 2},
		{"BIT", ModeAbsolute, 0x2C, 3},

		// BRK
		{"BRK", ModeImplied, 0x00, 1},

		// CLC, CLD, CLI, CLV
		{"CLC", ModeImplied, 0x18, 1},
		{"CLD", ModeImplied, 0xD8, 1},
		{"CLI", ModeImplied, 0x58, 1},
		{"CLV", ModeImplied, 0xB8, 1},

		// CMP
		{"CMP", ModeImmediate, 0xC9, 2},
		{"CMP", ModeZeroPage, 0xC5, 2},
		{"CMP", ModeZeroPageX, 0xD5, 2},
		{"CMP", ModeAbsolute, 0xCD, 3},
		{"CMP", ModeAbsoluteX, 0xDD, 3},
		{"CMP", ModeAbsoluteY, 0xD9, 3},
		{"CMP", ModeIndirectX, 0xC1, 2},
		{"CMP", ModeIndirectY, 0xD1, 2},

		// CPX
		{"CPX", ModeImmediate, 0xE0, 2},
		{"CPX", ModeZeroPage, 0xE4, 2},
		{"CPX", ModeAbsolute, 0xEC, 3},

		// CPY
		{"CPY", ModeImmediate, 0xC0, 2},
		{"CPY", ModeZeroPage, 0xC4, 2},
		{"CPY", ModeAbsolute, 0xCC, 3},

		// DEC
		{"DEC", ModeZeroPage, 0xC6, 2},
		{"DEC", ModeZeroPageX, 0xD6, 2},
		{"DEC", ModeAbsolute, 0xCE, 3},
		{"DEC", ModeAbsoluteX, 0xDE, 3},

		// DEX, DEY
		{"DEX", ModeImplied, 0xCA, 1},
		{"DEY", ModeImplied, 0x88, 1},

		// EOR
		{"EOR", ModeImmediate, 0x49, 2},
		{"EOR", ModeZeroPage, 0x45, 2},
		{"EOR", ModeZeroPageX, 0x55, 2},
		{"EOR", ModeAbsolute, 0x4D, 3},
		{"EOR", ModeAbsoluteX, 0x5D, 3},
		{"EOR", ModeAbsoluteY, 0x59, 3},
		{"EOR", ModeIndirectX, 0x41, 2},
		{"EOR", ModeIndirectY, 0x51, 2},

		// INC
		{"INC", ModeZeroPage, 0xE6, 2},
		{"INC", ModeZeroPageX, 0xF6, 2},
		{"INC", ModeAbsolute, 0xEE, 3},
		{"INC", ModeAbsoluteX, 0xFE, 3},

		// INX, INY
		{"INX", ModeImplied, 0xE8, 1},
		{"INY", ModeImplied, 0xC8, 1},

		// JMP
		{"JMP", ModeAbsolute, 0x4C, 3},
		{"JMP", ModeIndirect, 0x6C, 3},

		// JSR
		{"JSR", ModeAbsolute, 0x20, 3},

		// LDA
		{"LDA", ModeImmediate, 0xA9, 2},
		{"LDA", ModeZeroPage, 0xA5, 2},
		{"LDA", ModeZeroPageX, 0xB5, 2},
		{"LDA", ModeAbsolute, 0xAD, 3},
		{"LDA", ModeAbsoluteX, 0xBD, 3},
		{"LDA", ModeAbsoluteY, 0xB9, 3},
		{"LDA", ModeIndirectX, 0xA1, 2},
		{"LDA", ModeIndirectY, 0xB1, 2},

		// LDX
		{"LDX", ModeImmediate, 0xA2, 2},
		{"LDX", ModeZeroPage, 0xA6, 2},
		{"LDX", ModeZeroPageY, 0xB6, 2},
		{"LDX", ModeAbsolute, 0xAE, 3},
		{"LDX", ModeAbsoluteY, 0xBE, 3},

		// LDY
		{"LDY", ModeImmediate, 0xA0, 2},
		{"LDY", ModeZeroPage, 0xA4, 2},
		{"LDY", ModeZeroPageX, 0xB4, 2},
		{"LDY", ModeAbsolute, 0xAC, 3},
		{"LDY", ModeAbsoluteX, 0xBC, 3},

		// LSR
		{"LSR", ModeAccumulator, 0x4A, 1},
		{"LSR", ModeImplied, 0x4A, 1},
		{"LSR", ModeZeroPage, 0x46, 2},
		{"LSR", ModeZeroPageX, 0x56, 2},
		{"LSR", ModeAbsolute, 0x4E, 3},
		{"LSR", ModeAbsoluteX, 0x5E, 3},

		// NOP
		{"NOP", ModeImplied, 0xEA, 1},

		// ORA
		{"ORA", ModeImmediate, 0x09, 2},
		{"ORA", ModeZeroPage, 0x05, 2},
		{"ORA", ModeZeroPageX, 0x15, 2},
		{"ORA", ModeAbsolute, 0x0D, 3},
		{"ORA", ModeAbsoluteX, 0x1D, 3},
		{"ORA", ModeAbsoluteY, 0x19, 3},
		{"ORA", ModeIndirectX, 0x01, 2},
		{"ORA", ModeIndirectY, 0x11, 2},

		// PHA, PHP, PLA, PLP
		{"PHA", ModeImplied, 0x48, 1},
		{"PHP", ModeImplied, 0x08, 1},
		{"PLA", ModeImplied, 0x68, 1},
		{"PLP", ModeImplied, 0x28, 1},

		// ROL
		{"ROL", ModeAccumulator, 0x2A, 1},
		{"ROL", ModeImplied, 0x2A, 1},
		{"ROL", ModeZeroPage, 0x26, 2},
		{"ROL", ModeZeroPageX, 0x36, 2},
		{"ROL", ModeAbsolute, 0x2E, 3},
		{"ROL", ModeAbsoluteX, 0x3E, 3},

		// ROR
		{"ROR", ModeAccumulator, 0x6A, 1},
		{"ROR", ModeImplied, 0x6A, 1},
		{"ROR", ModeZeroPage, 0x66, 2},
		{"ROR", ModeZeroPageX, 0x76, 2},
		{"ROR", ModeAbsolute, 0x6E, 3},
		{"ROR", ModeAbsoluteX, 0x7E, 3},

		// RTI, RTS
		{"RTI", ModeImplied, 0x40, 1},
		{"RTS", ModeImplied, 0x60, 1},

		// SBC
		{"SBC", ModeImmediate, 0xE9, 2},
		{"SBC", ModeZeroPage, 0xE5, 2},
		{"SBC", ModeZeroPageX, 0xF5, 2},
		{"SBC", ModeAbsolute, 0xED, 3},
		{"SBC", ModeAbsoluteX, 0xFD, 3},
		{"SBC", ModeAbsoluteY, 0xF9, 3},
		{"SBC", ModeIndirectX, 0xE1, 2},
		{"SBC", ModeIndirectY, 0xF1, 2},

		// SEC, SED, SEI
		{"SEC", ModeImplied, 0x38, 1},
		{"SED", ModeImplied, 0xF8, 1},
		{"SEI", ModeImplied, 0x78, 1},

		// STA
		{"STA", ModeZeroPage, 0x85, 2},
		{"STA", ModeZeroPageX, 0x95, 2},
		{"STA", ModeAbsolute, 0x8D, 3},
		{"STA", ModeAbsoluteX, 0x9D, 3},
		{"STA", ModeAbsoluteY, 0x99, 3},
		{"STA", ModeIndirectX, 0x81, 2},
		{"STA", ModeIndirectY, 0x91, 2},

		// STX
		{"STX", ModeZeroPage, 0x86, 2},
		{"STX", ModeZeroPageY, 0x96, 2},
		{"STX", ModeAbsolute, 0x8E, 3},

		// STY
		{"STY", ModeZeroPage, 0x84, 2},
		{"STY", ModeZeroPageX, 0x94, 2},
		{"STY", ModeAbsolute, 0x8C, 3},

		// TAX, TAY, TSX, TXA, TXS, TYA
		{"TAX", ModeImplied, 0xAA, 1},
		{"TAY", ModeImplied, 0xA8, 1},
		{"TSX", ModeImplied, 0xBA, 1},
		{"TXA", ModeImplied, 0x8A, 1},
		{"TXS", ModeImplied, 0x9A, 1},
		{"TYA", ModeImplied, 0x98, 1},
	}

	opcodeMap = make(map[opcodeKey]InstructionInfo)
	mnemonics = make(map[string]bool)
	branches  = make(map[string]bool)
)

func init() {
	for _, inst := range instructions {
		key := opcodeKey{strings.ToUpper(inst.Mnemonic), inst.Mode}
		opcodeMap[key] = inst
		mnemonics[strings.ToUpper(inst.Mnemonic)] = true
	}
	for _, b := range []string{"BCC", "BCS", "BEQ", "BMI", "BNE", "BPL", "BVC", "BVS"} {
		branches[b] = true
	}
}

func IsMnemonic(s string) bool {
	return mnemonics[strings.ToUpper(s)]
}

func IsBranch(mnemonic string) bool {
	return branches[strings.ToUpper(mnemonic)]
}

func LookupInstruction(mnemonic string, mode AddressingMode) (InstructionInfo, bool) {
	inst, ok := opcodeMap[opcodeKey{strings.ToUpper(mnemonic), mode}]
	return inst, ok
}
