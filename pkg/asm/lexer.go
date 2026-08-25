package asm

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type Lexer struct {
	filename string
	input    []rune
	pos      int
	line     int
	col      int
}

func NewLexer(filename string, input string) *Lexer {
	return &Lexer{
		filename: filename,
		input:    []rune(input),
		pos:      0,
		line:     1,
		col:      1,
	}
}

func (l *Lexer) currentPos() Position {
	return Position{
		Filename: l.filename,
		Line:     l.line,
		Column:   l.col,
	}
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *Lexer) peekAhead(offset int) rune {
	if l.pos+offset >= len(l.input) {
		return 0
	}
	return l.input[l.pos+offset]
}

func (l *Lexer) advance() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	r := l.input[l.pos]
	l.pos++
	if r == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return r
}

func (l *Lexer) NextToken() (Token, error) {
	for {
		l.skipWhitespaceExceptNewline()

		r := l.peek()
		if r == 0 {
			return Token{Type: TokenEOF, Pos: l.currentPos()}, nil
		}

		pos := l.currentPos()

		// Comments
		if r == ';' {
			for l.peek() != 0 && l.peek() != '\n' {
				l.advance()
			}
			continue
		}

		// Newline / EOL
		if r == '\n' {
			l.advance()
			return Token{Type: TokenEOL, Literal: "\n", Pos: pos}, nil
		}
		if r == '\r' {
			l.advance()
			if l.peek() == '\n' {
				l.advance()
			}
			return Token{Type: TokenEOL, Literal: "\n", Pos: pos}, nil
		}

		// Anonymous label reference or definition: :+, :-, :
		if r == ':' {
			next := l.peekAhead(1)
			if next == '+' || next == '-' {
				l.advance() // consume ':'
				var count int
				isPlus := next == '+'
				for (isPlus && l.peek() == '+') || (!isPlus && l.peek() == '-') {
					l.advance()
					count++
				}
				lit := ":" + strings.Repeat(string(next), count)
				return Token{Type: TokenAnonRef, Literal: lit, NumValue: int64(count), Pos: pos}, nil
			}
			l.advance()
			return Token{Type: TokenColon, Literal: ":", Pos: pos}, nil
		}

		// Hash (Immediate)
		if r == '#' {
			l.advance()
			return Token{Type: TokenHash, Literal: "#", Pos: pos}, nil
		}

		// Local identifier or label: @name
		if r == '@' {
			l.advance()
			ident := l.readIdentifier()
			if ident == "" {
				return Token{}, fmt.Errorf("%s: expected identifier after '@'", pos)
			}
			return Token{Type: TokenLocalIdent, Literal: "@" + ident, Pos: pos}, nil
		}

		// Directives starting with dot '.'
		if r == '.' {
			l.advance()
			name := l.readIdentifier()
			tokType := lookupDirective("." + strings.ToLower(name))
			if tokType == TokenIdent {
				return Token{}, fmt.Errorf("%s: unknown directive '.%s'", pos, name)
			}
			return Token{Type: tokType, Literal: "." + name, Pos: pos}, nil
		}

		// Size overrides: z:, a:
		if (r == 'z' || r == 'Z' || r == 'a' || r == 'A') && l.peekAhead(1) == ':' {
			l.advance()
			l.advance()
			if r == 'z' || r == 'Z' {
				return Token{Type: TokenZPrefix, Literal: "z:", Pos: pos}, nil
			}
			return Token{Type: TokenAPrefix, Literal: "a:", Pos: pos}, nil
		}

		// Character literals
		if r == '\'' {
			charVal, err := l.readCharLiteral()
			if err != nil {
				return Token{}, err
			}
			return Token{Type: TokenChar, NumValue: int64(charVal), Literal: string(charVal), Pos: pos}, nil
		}

		// String literals
		if r == '"' {
			strVal, err := l.readStringLiteral()
			if err != nil {
				return Token{}, err
			}
			return Token{Type: TokenString, Literal: strVal, Pos: pos}, nil
		}

		// Hex numbers starting with '$'
		if r == '$' {
			l.advance()
			hexStr := l.readHexDigits()
			if hexStr == "" {
				return Token{}, fmt.Errorf("%s: expected hexadecimal digits after '$'", pos)
			}
			val, err := strconv.ParseInt(hexStr, 16, 64)
			if err != nil {
				return Token{}, fmt.Errorf("%s: invalid hexadecimal number: %w", pos, err)
			}
			return Token{Type: TokenNumber, NumValue: val, Literal: "$" + hexStr, Pos: pos}, nil
		}

		// Binary numbers starting with '%'
		if r == '%' {
			// Check if this is '%' modulo operator or binary literal
			// If followed immediately by '0' or '1', treat as binary number
			next := l.peekAhead(1)
			if next == '0' || next == '1' {
				l.advance()
				binStr := l.readBinDigits()
				val, err := strconv.ParseInt(binStr, 2, 64)
				if err != nil {
					return Token{}, fmt.Errorf("%s: invalid binary number: %w", pos, err)
				}
				return Token{Type: TokenNumber, NumValue: val, Literal: "%" + binStr, Pos: pos}, nil
			}
			l.advance()
			return Token{Type: TokenPercent, Literal: "%", Pos: pos}, nil
		}

		// Multi-character operators and single operators
		switch r {
		case ',':
			l.advance()
			return Token{Type: TokenComma, Literal: ",", Pos: pos}, nil
		case '(':
			l.advance()
			return Token{Type: TokenLParen, Literal: "(", Pos: pos}, nil
		case ')':
			l.advance()
			return Token{Type: TokenRParen, Literal: ")", Pos: pos}, nil
		case '[':
			l.advance()
			return Token{Type: TokenLBracket, Literal: "[", Pos: pos}, nil
		case ']':
			l.advance()
			return Token{Type: TokenRBracket, Literal: "]", Pos: pos}, nil
		case '+':
			l.advance()
			return Token{Type: TokenPlus, Literal: "+", Pos: pos}, nil
		case '-':
			l.advance()
			return Token{Type: TokenMinus, Literal: "-", Pos: pos}, nil
		case '*':
			l.advance()
			return Token{Type: TokenStar, Literal: "*", Pos: pos}, nil
		case '/':
			l.advance()
			return Token{Type: TokenSlash, Literal: "/", Pos: pos}, nil
		case '~':
			l.advance()
			return Token{Type: TokenTilde, Literal: "~", Pos: pos}, nil
		case '^':
			l.advance()
			return Token{Type: TokenCaret, Literal: "^", Pos: pos}, nil
		case '&':
			l.advance()
			if l.peek() == '&' {
				l.advance()
				return Token{Type: TokenLogicalAnd, Literal: "&&", Pos: pos}, nil
			}
			return Token{Type: TokenAmp, Literal: "&", Pos: pos}, nil
		case '|':
			l.advance()
			if l.peek() == '|' {
				l.advance()
				return Token{Type: TokenLogicalOr, Literal: "||", Pos: pos}, nil
			}
			return Token{Type: TokenPipe, Literal: "|", Pos: pos}, nil
		case '=':
			l.advance()
			if l.peek() == '=' {
				l.advance()
				return Token{Type: TokenEqEq, Literal: "==", Pos: pos}, nil
			}
			return Token{Type: TokenAssign, Literal: "=", Pos: pos}, nil
		case '!':
			l.advance()
			if l.peek() == '=' {
				l.advance()
				return Token{Type: TokenBangEq, Literal: "!=", Pos: pos}, nil
			}
			return Token{Type: TokenBang, Literal: "!", Pos: pos}, nil
		case '<':
			l.advance()
			if l.peek() == '<' {
				l.advance()
				return Token{Type: TokenLShift, Literal: "<<", Pos: pos}, nil
			}
			if l.peek() == '=' {
				l.advance()
				return Token{Type: TokenLtEq, Literal: "<=", Pos: pos}, nil
			}
			if l.peek() == '>' {
				l.advance()
				return Token{Type: TokenNotEqCa65, Literal: "<>", Pos: pos}, nil
			}
			return Token{Type: TokenLt, Literal: "<", Pos: pos}, nil
		case '>':
			l.advance()
			if l.peek() == '>' {
				l.advance()
				return Token{Type: TokenRShift, Literal: ">>", Pos: pos}, nil
			}
			if l.peek() == '=' {
				l.advance()
				return Token{Type: TokenGtEq, Literal: ">=", Pos: pos}, nil
			}
			return Token{Type: TokenGt, Literal: ">", Pos: pos}, nil
		}

		// Numbers: 0x... 0b... or decimal digits
		if unicode.IsDigit(r) {
			if r == '0' && (l.peekAhead(1) == 'x' || l.peekAhead(1) == 'X') {
				l.advance() // '0'
				l.advance() // 'x'
				hexStr := l.readHexDigits()
				val, err := strconv.ParseInt(hexStr, 16, 64)
				if err != nil {
					return Token{}, fmt.Errorf("%s: invalid hexadecimal number: %w", pos, err)
				}
				return Token{Type: TokenNumber, NumValue: val, Literal: "0x" + hexStr, Pos: pos}, nil
			}
			if r == '0' && (l.peekAhead(1) == 'b' || l.peekAhead(1) == 'B') {
				l.advance() // '0'
				l.advance() // 'b'
				binStr := l.readBinDigits()
				val, err := strconv.ParseInt(binStr, 2, 64)
				if err != nil {
					return Token{}, fmt.Errorf("%s: invalid binary number: %w", pos, err)
				}
				return Token{Type: TokenNumber, NumValue: val, Literal: "0b" + binStr, Pos: pos}, nil
			}

			decStr := l.readDecDigits()
			val, err := strconv.ParseInt(decStr, 10, 64)
			if err != nil {
				return Token{}, fmt.Errorf("%s: invalid decimal number: %w", pos, err)
			}
			return Token{Type: TokenNumber, NumValue: val, Literal: decStr, Pos: pos}, nil
		}

		// Identifiers and Registers
		if isIdentStart(r) {
			ident := l.readIdentifier()
			u := strings.ToUpper(ident)
			// Registers A, X, Y
			if len(ident) == 1 {
				switch u {
				case "A":
					return Token{Type: TokenRegA, Literal: ident, Pos: pos}, nil
				case "X":
					return Token{Type: TokenRegX, Literal: ident, Pos: pos}, nil
				case "Y":
					return Token{Type: TokenRegY, Literal: ident, Pos: pos}, nil
				}
			}
			return Token{Type: TokenIdent, Literal: ident, Pos: pos}, nil
		}

		return Token{}, fmt.Errorf("%s: unexpected character '%c' (U+%04X)", pos, r, r)
	}
}

func (l *Lexer) skipWhitespaceExceptNewline() {
	for {
		r := l.peek()
		if r == ' ' || r == '\t' || r == '\v' || r == '\f' {
			l.advance()
		} else {
			break
		}
	}
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentPart(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func (l *Lexer) readIdentifier() string {
	var sb strings.Builder
	for isIdentPart(l.peek()) {
		sb.WriteRune(l.advance())
	}
	return sb.String()
}

func (l *Lexer) readHexDigits() string {
	var sb strings.Builder
	for {
		r := l.peek()
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			sb.WriteRune(l.advance())
		} else {
			break
		}
	}
	return sb.String()
}

func (l *Lexer) readBinDigits() string {
	var sb strings.Builder
	for {
		r := l.peek()
		if r == '0' || r == '1' {
			sb.WriteRune(l.advance())
		} else {
			break
		}
	}
	return sb.String()
}

func (l *Lexer) readDecDigits() string {
	var sb strings.Builder
	for unicode.IsDigit(l.peek()) {
		sb.WriteRune(l.advance())
	}
	return sb.String()
}

func (l *Lexer) readCharLiteral() (rune, error) {
	pos := l.currentPos()
	l.advance() // consume initial '\''
	r := l.advance()
	if r == 0 || r == '\n' {
		return 0, fmt.Errorf("%s: unterminated character literal", pos)
	}
	var val rune
	if r == '\\' {
		esc := l.advance()
		switch esc {
		case 'n':
			val = '\n'
		case 'r':
			val = '\r'
		case 't':
			val = '\t'
		case '0':
			val = 0
		case '\\':
			val = '\\'
		case '\'':
			val = '\''
		case '"':
			val = '"'
		default:
			return 0, fmt.Errorf("%s: unknown escape sequence '\\%c'", pos, esc)
		}
	} else {
		val = r
	}
	if l.peek() != '\'' {
		return 0, fmt.Errorf("%s: missing closing quote for character literal", pos)
	}
	l.advance() // consume closing '\''
	return val, nil
}

func (l *Lexer) readStringLiteral() (string, error) {
	pos := l.currentPos()
	l.advance() // consume initial '"'
	var sb strings.Builder
	for {
		r := l.peek()
		if r == 0 || r == '\n' {
			return "", fmt.Errorf("%s: unterminated string literal", pos)
		}
		if r == '"' {
			l.advance()
			break
		}
		if r == '\\' {
			l.advance()
			esc := l.advance()
			switch esc {
			case 'n':
				sb.WriteRune('\n')
			case 'r':
				sb.WriteRune('\r')
			case 't':
				sb.WriteRune('\t')
			case '0':
				sb.WriteByte(0)
			case '\\':
				sb.WriteRune('\\')
			case '"':
				sb.WriteRune('"')
			case '\'':
				sb.WriteRune('\'')
			default:
				return "", fmt.Errorf("%s: unknown escape sequence '\\%c'", pos, esc)
			}
		} else {
			sb.WriteRune(l.advance())
		}
	}
	return sb.String(), nil
}

func lookupDirective(name string) TokenType {
	switch strings.ToLower(name) {
	case ".bank":
		return TokenDotBank
	case ".zp", ".zeropage":
		return TokenDotZP
	case ".ram", ".bss":
		return TokenDotRAM
	case ".wram", ".prgram", ".sram":
		return TokenDotWRAM
	case ".byte", ".byt", ".db":
		return TokenDotByte
	case ".word", ".addr", ".dw":
		return TokenDotWord
	case ".dword", ".dd":
		return TokenDotDword
	case ".asciiz", ".stringz":
		return TokenDotAsciiz
	case ".res", ".reserve":
		return TokenDotRes
	case ".export", ".global":
		return TokenDotExport
	case ".import":
		return TokenDotImport
	case ".importzp":
		return TokenDotImportZP
	case ".proc":
		return TokenDotProc
	case ".endproc":
		return TokenDotEndProc
	case ".scope":
		return TokenDotScope
	case ".endscope":
		return TokenDotEndScope
	case ".if":
		return TokenDotIf
	case ".ifdef":
		return TokenDotIfdef
	case ".ifndef":
		return TokenDotIfndef
	case ".elseif":
		return TokenDotElseif
	case ".else":
		return TokenDotElse
	case ".endif":
		return TokenDotEndif
	case ".macro":
		return TokenDotMacro
	case ".endmacro":
		return TokenDotEndMacro
	case ".include":
		return TokenDotInclude
	case ".incbin":
		return TokenDotIncbin
	case ".incchr":
		return TokenDotIncchr
	case ".incpal":
		return TokenDotIncpal
	case ".set":
		return TokenDotSet
	case ".equ":
		return TokenDotEqu
	default:
		return TokenIdent
	}
}
