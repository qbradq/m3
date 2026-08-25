package asm

import (
	"testing"
)

func TestLexerTokens(t *testing.T) {
	src := `
; Sample comment
start:
    LDA #$10
    STA $2000, X
    JSR wait_vblank
@loop:
    BEQ @loop
:   BIT $2002
    BPL :-
    BNE :+
:   RTS

.bank 2
.byte $01, $02, "ABC", 0
.word $8000, start
.asciiz "TEST"
.res 16, $FF
VAL = 42 + 2 * 3
`
	lexer := NewLexer("test.m3", src)
	for {
		tok, err := lexer.NextToken()
		if err != nil {
			t.Fatalf("unexpected lex error: %v", err)
		}
		if tok.Type == TokenEOF {
			break
		}
	}
}

func TestLexerNumbersAndStrings(t *testing.T) {
	src := `$FF %10101010 0x1A 0b11 255 'A' "Hello\nWorld"`
	lexer := NewLexer("test.m3", src)

	expected := []struct {
		typ TokenType
		val int64
		lit string
	}{
		{TokenNumber, 0xFF, "$FF"},
		{TokenNumber, 170, "%10101010"},
		{TokenNumber, 0x1A, "0x1A"},
		{TokenNumber, 3, "0b11"},
		{TokenNumber, 255, "255"},
		{TokenChar, 'A', "A"},
		{TokenString, 0, "Hello\nWorld"},
	}

	for i, exp := range expected {
		tok, err := lexer.NextToken()
		if err != nil {
			t.Fatalf("[%d] lexer error: %v", i, err)
		}
		if tok.Type != exp.typ {
			t.Errorf("[%d] expected token type %v, got %v", i, exp.typ, tok.Type)
		}
		if exp.typ == TokenNumber || exp.typ == TokenChar {
			if tok.NumValue != exp.val {
				t.Errorf("[%d] expected value %d, got %d", i, exp.val, tok.NumValue)
			}
		}
		if exp.typ == TokenString && tok.Literal != exp.lit {
			t.Errorf("[%d] expected string %q, got %q", i, exp.lit, tok.Literal)
		}
	}
}
