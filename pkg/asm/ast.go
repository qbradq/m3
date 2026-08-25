package asm

import "github.com/qbradq/m3/pkg/cpu6502"

type Statement interface {
	Pos() Position
}

// LabelStmt defines a global, local, or anonymous label
type LabelStmt struct {
	Name      string
	IsLocal   bool
	IsAnon    bool
	pos       Position
}

func (l *LabelStmt) Pos() Position { return l.pos }

// AssignmentStmt: IDENT = expr or IDENT .set expr
type AssignmentStmt struct {
	Name   string
	Value  Expr
	IsSet  bool
	pos    Position
}

func (a *AssignmentStmt) Pos() Position { return a.pos }

// InstructionStmt represents a 6502 machine instruction
type InstructionStmt struct {
	Mnemonic string
	Mode     cpu6502.AddressingMode
	Operand  Expr
	pos      Position
}

func (i *InstructionStmt) Pos() Position { return i.pos }

// BankDirective: .bank <expr>
type BankDirective struct {
	BankIndex Expr
	pos       Position
}

func (b *BankDirective) Pos() Position { return b.pos }

// MemorySegment indicates which memory segment is active
type MemorySegment int

const (
	SegmentPRG MemorySegment = iota
	SegmentZP
	SegmentRAM
	SegmentWRAM
)

// MemoryDirective: .zp, .ram, .wram with optional size: .zp [<size>]
type MemoryDirective struct {
	Segment MemorySegment
	Size    Expr // nil if no size specified
	pos     Position
}

func (m *MemoryDirective) Pos() Position { return m.pos }

// DataType for .byte, .word, .dword, .asciiz
type DataType int

const (
	DataByte DataType = iota
	DataWord
	DataDword
	DataAsciiz
)

type DataItem struct {
	Expr   Expr
	String string
	IsStr  bool
}

type DataDirective struct {
	Type  DataType
	Items []DataItem
	pos   Position
}

func (d *DataDirective) Pos() Position { return d.pos }

// ReserveDirective: .res <size> [, <val>]
type ReserveDirective struct {
	Size Expr
	Fill Expr
	pos  Position
}

func (r *ReserveDirective) Pos() Position { return r.pos }

// ExportDirective: .export ident1, ident2
type ExportDirective struct {
	Names []string
	pos   Position
}

func (e *ExportDirective) Pos() Position { return e.pos }

// ImportDirective: .import / .importzp ident1, ident2
type ImportDirective struct {
	Names      []string
	IsZeroPage bool
	pos        Position
}

func (i *ImportDirective) Pos() Position { return i.pos }

// ScopeDirective: .proc, .endproc, .scope, .endscope
type ScopeDirective struct {
	Name   string
	IsProc bool
	IsEnd  bool
	pos    Position
}

func (s *ScopeDirective) Pos() Position { return s.pos }

// IncludeDirective: .include / .incbin
type IncludeDirective struct {
	Filename string
	IsBin    bool
	Offset   Expr
	Size     Expr
	pos      Position
}

func (i *IncludeDirective) Pos() Position { return i.pos }

// IncchrDirective: .incchr <filename>
type IncchrDirective struct {
	Filename string
	pos      Position
}

func (i *IncchrDirective) Pos() Position { return i.pos }

// IncpalDirective: .incpal <filename> [, <count>]
type IncpalDirective struct {
	Filename string
	Count    Expr
	pos      Position
}

func (i *IncpalDirective) Pos() Position { return i.pos }

