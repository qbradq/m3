package compiler

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lexer converts source code into a stream of tokens.
type Lexer struct {
	filename   string
	source     string
	ch         rune
	offset     int // current character offset
	readOffset int // next character offset
	line       int // current line (1-based)
	col        int // current column (1-based)

	lastTokenType  TokenType
	insertSemicolon bool
	pragmas        []string
}

// NewLexer creates a new Lexer for the given source code.
func NewLexer(filename, source string) *Lexer {
	l := &Lexer{
		filename: filename,
		source:   source,
		line:     1,
		col:      0,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readOffset >= len(l.source) {
		l.ch = 0
		l.offset = len(l.source)
	} else {
		var size int
		l.ch, size = utf8.DecodeRuneInString(l.source[l.readOffset:])
		l.offset = l.readOffset
		l.readOffset += size
		l.col++
	}
}

func (l *Lexer) peekChar() rune {
	if l.readOffset >= len(l.source) {
		return 0
	}
	ch, _ := utf8.DecodeRuneInString(l.source[l.readOffset:])
	return ch
}

// NextToken returns the next token from the source.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	if l.insertSemicolon {
		l.insertSemicolon = false
		return Token{
			Type:    TokenSemicolon,
			Literal: ";",
			Pos:     Position{Filename: l.filename, Line: l.line, Column: l.col},
		}
	}

	pos := Position{Filename: l.filename, Line: l.line, Column: l.col}

	if l.ch == 0 {
		if l.shouldInsertSemicolon(l.lastTokenType) {
			l.lastTokenType = TokenEOF
			return Token{Type: TokenSemicolon, Literal: ";", Pos: pos}
		}
		return Token{Type: TokenEOF, Literal: "", Pos: pos}
	}

	var tok Token

	switch l.ch {
	case '\n':
		l.readChar()
		l.line++
		l.col = 0
		if l.shouldInsertSemicolon(l.lastTokenType) {
			l.lastTokenType = TokenSemicolon
			return Token{Type: TokenSemicolon, Literal: "\n", Pos: pos}
		}
		return l.NextToken()

	case '+':
		if l.peekChar() == '+' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenPlusPlus, Literal: "++", Pos: pos}
		} else if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenPlusEq, Literal: "+=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenPlus, Literal: "+", Pos: pos}
		}

	case '-':
		if l.peekChar() == '-' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenMinusMinus, Literal: "--", Pos: pos}
		} else if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenMinusEq, Literal: "-=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenMinus, Literal: "-", Pos: pos}
		}

	case '*':
		if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenStarEq, Literal: "*=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenStar, Literal: "*", Pos: pos}
		}

	case '/':
		if l.peekChar() == '/' {
			// Line comment
			l.readLineComment()
			return l.NextToken()
		} else if l.peekChar() == '*' {
			// Block comment
			l.readBlockComment()
			return l.NextToken()
		} else if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenSlashEq, Literal: "/=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenSlash, Literal: "/", Pos: pos}
		}

	case '%':
		// Could be modulo or binary literal like %10101010
		if isBinaryDigit(l.peekChar()) {
			tok = l.readBinaryLiteral(pos)
		} else {
			l.readChar()
			tok = Token{Type: TokenPercent, Literal: "%", Pos: pos}
		}

	case '$':
		// Hexadecimal literal $FF, $8000
		tok = l.readHexLiteral(pos)

	case '&':
		if l.peekChar() == '&' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenLogicalAnd, Literal: "&&", Pos: pos}
		} else if l.peekChar() == '^' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				l.readChar()
				tok = Token{Type: TokenAmpCaretEq, Literal: "&^=", Pos: pos}
			} else {
				l.readChar()
				tok = Token{Type: TokenAmpCaret, Literal: "&^", Pos: pos}
			}
		} else if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenAmpEq, Literal: "&=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenAmp, Literal: "&", Pos: pos}
		}

	case '|':
		if l.peekChar() == '|' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenLogicalOr, Literal: "||", Pos: pos}
		} else if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenPipeEq, Literal: "|=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenPipe, Literal: "|", Pos: pos}
		}

	case '^':
		if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenCaretEq, Literal: "^=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenCaret, Literal: "^", Pos: pos}
		}

	case '~':
		l.readChar()
		tok = Token{Type: TokenTilde, Literal: "~", Pos: pos}

	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenBangEq, Literal: "!=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenBang, Literal: "!", Pos: pos}
		}

	case '<':
		if l.peekChar() == '<' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				l.readChar()
				tok = Token{Type: TokenLShiftEq, Literal: "<<=", Pos: pos}
			} else {
				l.readChar()
				tok = Token{Type: TokenLShift, Literal: "<<", Pos: pos}
			}
		} else if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenLtEq, Literal: "<=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenLt, Literal: "<", Pos: pos}
		}

	case '>':
		if l.peekChar() == '>' {
			l.readChar()
			if l.peekChar() == '=' {
				l.readChar()
				l.readChar()
				tok = Token{Type: TokenRShiftEq, Literal: ">>=", Pos: pos}
			} else {
				l.readChar()
				tok = Token{Type: TokenRShift, Literal: ">>", Pos: pos}
			}
		} else if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenGtEq, Literal: ">=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenGt, Literal: ">", Pos: pos}
		}

	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenEqEq, Literal: "==", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenEq, Literal: "=", Pos: pos}
		}

	case ':':
		if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			tok = Token{Type: TokenColonEq, Literal: ":=", Pos: pos}
		} else {
			l.readChar()
			tok = Token{Type: TokenColon, Literal: ":", Pos: pos}
		}

	case '(':
		l.readChar()
		tok = Token{Type: TokenLParen, Literal: "(", Pos: pos}
	case ')':
		l.readChar()
		tok = Token{Type: TokenRParen, Literal: ")", Pos: pos}
	case '[':
		l.readChar()
		tok = Token{Type: TokenLBracket, Literal: "[", Pos: pos}
	case ']':
		l.readChar()
		tok = Token{Type: TokenRBracket, Literal: "]", Pos: pos}
	case '{':
		l.readChar()
		tok = Token{Type: TokenLBrace, Literal: "{", Pos: pos}
	case '}':
		l.readChar()
		tok = Token{Type: TokenRBrace, Literal: "}", Pos: pos}
	case ',':
		l.readChar()
		tok = Token{Type: TokenComma, Literal: ",", Pos: pos}
	case ';':
		l.readChar()
		tok = Token{Type: TokenSemicolon, Literal: ";", Pos: pos}
	case '.':
		l.readChar()
		tok = Token{Type: TokenDot, Literal: ".", Pos: pos}

	case '"':
		tok = l.readString(pos)

	case '\'':
		tok = l.readCharLiteral(pos)

	default:
		if isLetter(l.ch) || l.ch == '_' {
			tok = l.readIdentifier(pos)
		} else if isDigit(l.ch) {
			tok = l.readDecimalOrPrefixedNumber(pos)
		} else {
			ch := l.ch
			l.readChar()
			tok = Token{
				Type:    TokenError,
				Literal: fmt.Sprintf("illegal character '%c' (0x%X)", ch, ch),
				Pos:     pos,
			}
		}
	}

	l.lastTokenType = tok.Type
	return tok
}

func (l *Lexer) shouldInsertSemicolon(last TokenType) bool {
	switch last {
	case TokenIdent, TokenNumber, TokenString, TokenChar,
		TokenTrue, TokenFalse, TokenBreak, TokenContinue,
		TokenReturn, TokenPlusPlus, TokenMinusMinus,
		TokenRParen, TokenRBracket, TokenRBrace:
		return true
	default:
		return false
	}
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) readLineComment() {
	// Skip '//'
	l.readChar()
	l.readChar()

	start := l.offset
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
	commentText := strings.TrimSpace(l.source[start:l.offset])

	// Check if this is a pragma like `//export <name>`
	if strings.HasPrefix(commentText, "export ") || strings.HasPrefix(commentText, "export\t") {
		l.pragmas = append(l.pragmas, commentText)
	}
}

func (l *Lexer) readBlockComment() {
	// Skip '/*'
	l.readChar()
	l.readChar()

	for l.ch != 0 {
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar()
			l.readChar()
			return
		}
		if l.ch == '\n' {
			l.line++
			l.col = 0
		}
		l.readChar()
	}
}

// TakePragmas returns collected pragmas and clears the buffer.
func (l *Lexer) TakePragmas() []string {
	p := l.pragmas
	l.pragmas = nil
	return p
}

func (l *Lexer) skipWhitespaceAndNewlines() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' || l.ch == '\n' {
		if l.ch == '\n' {
			l.line++
			l.col = 0
		}
		l.readChar()
	}
}

func (l *Lexer) readIdentifier(pos Position) Token {
	start := l.offset
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	literal := l.source[start:l.offset]
	tokType := LookupIdent(literal)

	if tokType == TokenAsm {
		l.skipWhitespaceAndNewlines()
		if l.ch == '{' {
			body, err := l.scanAsmBlock(pos)
			if err != nil {
				return Token{Type: TokenError, Literal: err.Error(), Pos: pos}
			}
			return Token{Type: TokenAsm, Literal: body, Pos: pos}
		}
	}

	return Token{
		Type:    tokType,
		Literal: literal,
		Pos:     pos,
	}
}

func (l *Lexer) scanAsmBlock(pos Position) (string, error) {
	if l.ch != '{' {
		return "", fmt.Errorf("%s: expected '{' to start asm block, got '%c'", pos, l.ch)
	}
	l.readChar() // skip opening '{'

	braceDepth := 1
	var sb strings.Builder

	for l.ch != 0 {
		if l.ch == '{' {
			braceDepth++
			sb.WriteRune(l.ch)
		} else if l.ch == '}' {
			braceDepth--
			if braceDepth == 0 {
				l.readChar() // consume closing '}'
				return sb.String(), nil
			}
			sb.WriteRune(l.ch)
		} else {
			if l.ch == '\n' {
				l.line++
				l.col = 0
			}
			sb.WriteRune(l.ch)
		}
		l.readChar()
	}

	return "", fmt.Errorf("%s: unterminated asm block (missing closing '}')", pos)
}

func (l *Lexer) readHexLiteral(pos Position) Token {
	// Skip '$'
	l.readChar()
	start := l.offset
	for isHexDigit(l.ch) {
		l.readChar()
	}
	hexStr := l.source[start:l.offset]
	if len(hexStr) == 0 {
		return Token{Type: TokenError, Literal: "expected hex digits after '$'", Pos: pos}
	}
	val, err := strconv.ParseInt(hexStr, 16, 64)
	if err != nil {
		return Token{Type: TokenError, Literal: fmt.Sprintf("invalid hex number: %s", hexStr), Pos: pos}
	}
	return Token{
		Type:     TokenNumber,
		Literal:  "$" + hexStr,
		IntValue: val,
		Pos:      pos,
	}
}

func (l *Lexer) readBinaryLiteral(pos Position) Token {
	// Skip '%'
	l.readChar()
	start := l.offset
	for isBinaryDigit(l.ch) {
		l.readChar()
	}
	binStr := l.source[start:l.offset]
	val, err := strconv.ParseInt(binStr, 2, 64)
	if err != nil {
		return Token{Type: TokenError, Literal: fmt.Sprintf("invalid binary number: %s", binStr), Pos: pos}
	}
	return Token{
		Type:     TokenNumber,
		Literal:  "%" + binStr,
		IntValue: val,
		Pos:      pos,
	}
}

func (l *Lexer) readDecimalOrPrefixedNumber(pos Position) Token {
	start := l.offset

	// Check for 0x (hex) or 0b (bin)
	if l.ch == '0' {
		peek := l.peekChar()
		if peek == 'x' || peek == 'X' {
			l.readChar() // 0
			l.readChar() // x
			hexStart := l.offset
			for isHexDigit(l.ch) {
				l.readChar()
			}
			hexStr := l.source[hexStart:l.offset]
			if len(hexStr) == 0 {
				return Token{Type: TokenError, Literal: "expected hex digits after 0x", Pos: pos}
			}
			val, err := strconv.ParseInt(hexStr, 16, 64)
			if err != nil {
				return Token{Type: TokenError, Literal: fmt.Sprintf("invalid hex number: %s", hexStr), Pos: pos}
			}
			return Token{
				Type:     TokenNumber,
				Literal:  l.source[start:l.offset],
				IntValue: val,
				Pos:      pos,
			}
		} else if peek == 'b' || peek == 'B' {
			l.readChar() // 0
			l.readChar() // b
			binStart := l.offset
			for isBinaryDigit(l.ch) {
				l.readChar()
			}
			binStr := l.source[binStart:l.offset]
			if len(binStr) == 0 {
				return Token{Type: TokenError, Literal: "expected binary digits after 0b", Pos: pos}
			}
			val, err := strconv.ParseInt(binStr, 2, 64)
			if err != nil {
				return Token{Type: TokenError, Literal: fmt.Sprintf("invalid binary number: %s", binStr), Pos: pos}
			}
			return Token{
				Type:     TokenNumber,
				Literal:  l.source[start:l.offset],
				IntValue: val,
				Pos:      pos,
			}
		}
	}

	for isDigit(l.ch) {
		l.readChar()
	}
	literal := l.source[start:l.offset]
	val, err := strconv.ParseInt(literal, 10, 64)
	if err != nil {
		return Token{Type: TokenError, Literal: fmt.Sprintf("invalid integer: %s", literal), Pos: pos}
	}
	return Token{
		Type:     TokenNumber,
		Literal:  literal,
		IntValue: val,
		Pos:      pos,
	}
}

func (l *Lexer) readString(pos Position) Token {
	l.readChar() // skip opening quote
	var sb strings.Builder

	for l.ch != '"' && l.ch != 0 {
		if l.ch == '\n' {
			return Token{Type: TokenError, Literal: "unterminated string literal", Pos: pos}
		}
		if l.ch == '\\' {
			l.readChar()
			switch l.ch {
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			case '0':
				sb.WriteByte(0)
			case '\\':
				sb.WriteByte('\\')
			case '"':
				sb.WriteByte('"')
			case '\'':
				sb.WriteByte('\'')
			case 'x', 'X':
				l.readChar()
				h1 := l.ch
				l.readChar()
				h2 := l.ch
				hexStr := string([]rune{h1, h2})
				val, err := strconv.ParseInt(hexStr, 16, 8)
				if err != nil {
					return Token{Type: TokenError, Literal: fmt.Sprintf("invalid hex escape: \\x%s", hexStr), Pos: pos}
				}
				sb.WriteByte(byte(val))
			default:
				sb.WriteByte('\\')
				sb.WriteRune(l.ch)
			}
		} else {
			sb.WriteRune(l.ch)
		}
		l.readChar()
	}

	if l.ch != '"' {
		return Token{Type: TokenError, Literal: "unterminated string literal", Pos: pos}
	}
	l.readChar() // skip closing quote

	return Token{
		Type:    TokenString,
		Literal: sb.String(),
		Pos:     pos,
	}
}

func (l *Lexer) readCharLiteral(pos Position) Token {
	l.readChar() // skip opening quote
	var val rune

	if l.ch == '\'' || l.ch == 0 || l.ch == '\n' {
		return Token{Type: TokenError, Literal: "empty or unterminated character literal", Pos: pos}
	}

	if l.ch == '\\' {
		l.readChar()
		switch l.ch {
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
		case 'x', 'X':
			l.readChar()
			h1 := l.ch
			l.readChar()
			h2 := l.ch
			hexStr := string([]rune{h1, h2})
			hVal, err := strconv.ParseInt(hexStr, 16, 8)
			if err != nil {
				return Token{Type: TokenError, Literal: fmt.Sprintf("invalid hex escape: \\x%s", hexStr), Pos: pos}
			}
			val = rune(hVal)
		default:
			val = l.ch
		}
	} else {
		val = l.ch
	}
	l.readChar()

	if l.ch != '\'' {
		return Token{Type: TokenError, Literal: "unterminated character literal", Pos: pos}
	}
	l.readChar() // skip closing quote

	return Token{
		Type:     TokenChar,
		Literal:  string(val),
		IntValue: int64(val),
		Pos:      pos,
	}
}


func isLetter(ch rune) bool {
	return unicode.IsLetter(ch)
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

func isHexDigit(ch rune) bool {
	return isDigit(ch) || ('a' <= ch && ch <= 'f') || ('A' <= ch && ch <= 'F')
}

func isBinaryDigit(ch rune) bool {
	return ch == '0' || ch == '1'
}
