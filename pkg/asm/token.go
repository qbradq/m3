package asm

import "fmt"

type TokenType int

const (
	TokenEOF TokenType = iota
	TokenEOL
	TokenComment

	// Literals & Identifiers
	TokenIdent     // e.g. main, loop_1
	TokenLocalIdent // e.g. @loop
	TokenAnonLabel // :
	TokenAnonRef   // :+, :++, :-, :--
	TokenNumber    // 123, $10, %1010, 0x10, 0b1010
	TokenString    // "hello"
	TokenChar      // 'A'

	// Keywords / Registers
	TokenRegA // A or a
	TokenRegX // X or x
	TokenRegY // Y or y

	// Directives
	TokenDotBank    // .bank
	TokenDotZP      // .zp
	TokenDotRAM     // .ram
	TokenDotWRAM    // .wram
	TokenDotByte    // .byte, .byt, .db
	TokenDotWord    // .word, .addr, .dw
	TokenDotDword   // .dword, .dd
	TokenDotAsciiz  // .asciiz, .stringz
	TokenDotRes     // .res, .reserve
	TokenDotExport  // .export, .global
	TokenDotImport  // .import
	TokenDotImportZP// .importzp
	TokenDotProc    // .proc
	TokenDotEndProc // .endproc
	TokenDotScope   // .scope
	TokenDotEndScope// .endscope
	TokenDotIf      // .if
	TokenDotIfdef   // .ifdef
	TokenDotIfndef  // .ifndef
	TokenDotElseif  // .elseif
	TokenDotElse    // .else
	TokenDotEndif   // .endif
	TokenDotMacro   // .macro
	TokenDotEndMacro// .endmacro
	TokenDotInclude // .include
	TokenDotIncbin  // .incbin
	TokenDotIncchr  // .incchr
	TokenDotIncpal  // .incpal
	TokenDotSet     // .set
	TokenDotEqu     // .equ
	TokenDotDefine  // .define, .def

	// Delimiters & Punctuations
	TokenColon     // :
	TokenHash      // #
	TokenComma     // ,
	TokenLParen    // (
	TokenRParen    // )
	TokenLBracket  // [
	TokenRBracket  // ]
	TokenAssign    // =

	// Operators
	TokenPlus      // +
	TokenMinus     // -
	TokenStar      // *
	TokenSlash     // /
	TokenPercent   // %
	TokenTilde     // ~
	TokenBang      // !
	TokenAmp       // &
	TokenPipe      // |
	TokenCaret     // ^
	TokenLShift    // <<
	TokenRShift    // >>
	TokenLt        // <
	TokenGt        // >
	TokenLtEq      // <=
	TokenGtEq      // >=
	TokenEqEq      // ==
	TokenBangEq    // !=
	TokenNotEqCa65 // <>
	TokenLogicalAnd// &&
	TokenLogicalOr // ||

	// Size overrides
	TokenZPrefix   // z:
	TokenAPrefix   // a:
)

type Position struct {
	Filename string
	Line     int
	Column   int
}

func (p Position) String() string {
	if p.Filename == "" {
		return fmt.Sprintf("%d:%d", p.Line, p.Column)
	}
	return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
}

type Token struct {
	Type     TokenType
	Literal  string
	NumValue int64
	Pos      Position
}

func (t Token) String() string {
	if t.Literal != "" {
		return fmt.Sprintf("Token(%d, %q, %s)", t.Type, t.Literal, t.Pos)
	}
	return fmt.Sprintf("Token(%d, %s)", t.Type, t.Pos)
}
