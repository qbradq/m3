package compiler

import "fmt"

// TokenType represents a lexical token type.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenError
	TokenComment

	// Literals & Identifiers
	TokenIdent  // player_x, Enemy, count
	TokenNumber // 123, $FF, 0xFF, %1010, 0b1010
	TokenString // "hello"
	TokenChar   // 'A'

	// Keywords
	TokenPackage  // package
	TokenImport   // import
	TokenVar      // var
	TokenConst    // const
	TokenDefine   // define
	TokenTypeKw   // type
	TokenStruct   // struct
	TokenFunc     // func
	TokenIf       // if
	TokenElse     // else
	TokenFor      // for
	TokenSwitch   // switch
	TokenCase     // case
	TokenDefault  // default
	TokenReturn   // return
	TokenBreak    // break
	TokenContinue // continue
	TokenAsm      // asm

	// Storage & Banking Keywords
	TokenZP       // zp
	TokenZeroPage // zeropage
	TokenRAM      // ram
	TokenWRAM     // wram
	TokenWorkRAM  // workram
	TokenBank     // bank
	TokenAuto     // auto

	// Built-in Types & Boolean Literals
	TokenTrue  // true
	TokenFalse // false

	// Operators & Delimiters
	TokenPlus      // +
	TokenMinus     // -
	TokenStar      // *
	TokenSlash     // /
	TokenPercent   // %
	TokenAmp       // &
	TokenPipe      // |
	TokenCaret     // ^
	TokenAmpCaret  // &^
	TokenTilde     // ~
	TokenBang      // !
	TokenLShift    // <<
	TokenRShift    // >>
	TokenEq        // =
	TokenEqEq      // ==
	TokenBangEq    // !=
	TokenLt        // <
	TokenLtEq      // <=
	TokenGt        // >
	TokenGtEq      // >=
	TokenColonEq   // :=
	TokenPlusEq    // +=
	TokenMinusEq   // -=
	TokenStarEq    // *=
	TokenSlashEq   // /=
	TokenAmpEq     // &=
	TokenPipeEq    // |=
	TokenCaretEq   // ^=
	TokenAmpCaretEq// &^=
	TokenLShiftEq  // <<=
	TokenRShiftEq  // >>=
	TokenPlusPlus  // ++
	TokenMinusMinus// --
	TokenLogicalAnd// &&
	TokenLogicalOr // ||

	// Delimiters
	TokenLParen    // (
	TokenRParen    // )
	TokenLBracket  // [
	TokenRBracket  // ]
	TokenLBrace    // {
	TokenRBrace    // }
	TokenComma     // ,
	TokenSemicolon // ;
	TokenColon     // :
	TokenDot       // .
)

var tokenNames = map[TokenType]string{
	TokenEOF:        "EOF",
	TokenError:      "ERROR",
	TokenComment:    "COMMENT",
	TokenIdent:      "IDENT",
	TokenNumber:     "NUMBER",
	TokenString:     "STRING",
	TokenChar:       "CHAR",
	TokenPackage:    "package",
	TokenImport:     "import",
	TokenVar:        "var",
	TokenConst:      "const",
	TokenDefine:     "define",
	TokenTypeKw:     "type",
	TokenStruct:     "struct",
	TokenFunc:       "func",
	TokenIf:         "if",
	TokenElse:       "else",
	TokenFor:        "for",
	TokenSwitch:     "switch",
	TokenCase:       "case",
	TokenDefault:    "default",
	TokenReturn:     "return",
	TokenBreak:      "break",
	TokenContinue:   "continue",
	TokenAsm:        "asm",
	TokenZP:         "zp",
	TokenZeroPage:   "zeropage",
	TokenRAM:        "ram",
	TokenWRAM:       "wram",
	TokenWorkRAM:    "workram",
	TokenBank:       "bank",
	TokenAuto:       "auto",
	TokenTrue:       "true",
	TokenFalse:      "false",
	TokenPlus:        "+",
	TokenMinus:       "-",
	TokenStar:        "*",
	TokenSlash:       "/",
	TokenPercent:     "%",
	TokenAmp:         "&",
	TokenPipe:        "|",
	TokenCaret:       "^",
	TokenAmpCaret:    "&^",
	TokenTilde:       "~",
	TokenBang:        "!",
	TokenLShift:      "<<",
	TokenRShift:      ">>",
	TokenEq:          "=",
	TokenEqEq:        "==",
	TokenBangEq:      "!=",
	TokenLt:          "<",
	TokenLtEq:        "<=",
	TokenGt:          ">",
	TokenGtEq:        ">=",
	TokenColonEq:     ":=",
	TokenPlusEq:      "+=",
	TokenMinusEq:     "-=",
	TokenStarEq:      "*=",
	TokenSlashEq:     "/=",
	TokenAmpEq:       "&=",
	TokenPipeEq:      "|=",
	TokenCaretEq:     "^=",
	TokenAmpCaretEq:  "&^=",
	TokenLShiftEq:    "<<=",
	TokenRShiftEq:    ">>=",
	TokenPlusPlus:    "++",
	TokenMinusMinus:  "--",
	TokenLogicalAnd:  "&&",
	TokenLogicalOr:   "||",
	TokenLParen:      "(",
	TokenRParen:      ")",
	TokenLBracket:    "[",
	TokenRBracket:    "]",
	TokenLBrace:      "{",
	TokenRBrace:      "}",
	TokenComma:       ",",
	TokenSemicolon:   ";",
	TokenColon:       ":",
	TokenDot:         ".",
}

func (t TokenType) String() string {
	if name, ok := tokenNames[t]; ok {
		return name
	}
	return fmt.Sprintf("TOKEN(%d)", int(t))
}

// Position tracks location within source code.
type Position struct {
	Filename string
	Line     int
	Column   int
}

func (p Position) String() string {
	if p.Filename != "" {
		return fmt.Sprintf("%s:%d:%d", p.Filename, p.Line, p.Column)
	}
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Token represents a scanned token.
type Token struct {
	Type     TokenType
	Literal  string
	IntValue int64
	Pos      Position
}

func (t Token) String() string {
	return fmt.Sprintf("%v(%q) at %s", t.Type, t.Literal, t.Pos)
}

// Keywords map
var keywords = map[string]TokenType{
	"package":  TokenPackage,
	"import":   TokenImport,
	"var":      TokenVar,
	"const":    TokenConst,
	"define":   TokenDefine,
	"type":     TokenTypeKw,
	"struct":   TokenStruct,
	"func":     TokenFunc,
	"if":       TokenIf,
	"else":     TokenElse,
	"for":      TokenFor,
	"switch":   TokenSwitch,
	"case":     TokenCase,
	"default":  TokenDefault,
	"return":   TokenReturn,
	"break":    TokenBreak,
	"continue": TokenContinue,
	"asm":      TokenAsm,
	"zp":       TokenZP,
	"zeropage": TokenZeroPage,
	"ram":      TokenRAM,
	"wram":     TokenWRAM,
	"workram":  TokenWorkRAM,
	"bank":     TokenBank,
	"auto":     TokenAuto,
	"true":     TokenTrue,
	"false":    TokenFalse,
}

// LookupIdent checks if an identifier is a keyword.
func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TokenIdent
}
