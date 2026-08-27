package compiler

import (
	"testing"
)

func TestLexerTokens(t *testing.T) {
	input := `
	package main
	import "foo/bar.m3"
	var player_x uint8 zp
	var actors Actor[16] ram
	const MAX_LIVES uint8 = 3
	const palette uint8[16] bank 0 = [4]uint8{$0F, $00, $10, $30}
	type Actor struct {
		x uint8
		y uint8
	}
	func update() bank auto {
		if player_x > 240 {
			player_x = 240
		} else {
			player_x += 1
		}
		for i := uint8(0); i < 16; i++ {
			actors[i].x += 1
		}
	}
	`

	lexer := NewLexer("test.m3", input)
	var tokens []Token
	for {
		tok := lexer.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF {
			break
		}
	}

	if len(tokens) == 0 {
		t.Fatalf("expected tokens, got 0")
	}

	// Verify no error tokens
	for _, tok := range tokens {
		if tok.Type == TokenError {
			t.Errorf("unexpected error token: %v", tok)
		}
	}
}

func TestLexerNumbersAndLiterals(t *testing.T) {
	input := `123 $FF $8000 0xFF %10101010 0b1100 'A' '\n' '\0' "Hello World" true false`
	lexer := NewLexer("literals.m3", input)

	expected := []struct {
		tokType TokenType
		literal string
		val     int64
	}{
		{TokenNumber, "123", 123},
		{TokenNumber, "$FF", 255},
		{TokenNumber, "$8000", 32768},
		{TokenNumber, "0xFF", 255},
		{TokenNumber, "%10101010", 170},
		{TokenNumber, "0b1100", 12},
		{TokenChar, "A", 'A'},
		{TokenChar, "\n", '\n'},
		{TokenChar, "\x00", 0},
		{TokenString, "Hello World", 0},
		{TokenTrue, "true", 0},
		{TokenFalse, "false", 0},
	}

	for _, exp := range expected {
		tok := lexer.NextToken()
		if tok.Type != exp.tokType {
			t.Errorf("expected token type %v, got %v (%q)", exp.tokType, tok.Type, tok.Literal)
		}
		if exp.tokType == TokenNumber || exp.tokType == TokenChar {
			if tok.IntValue != exp.val {
				t.Errorf("expected int value %d, got %d", exp.val, tok.IntValue)
			}
		}
		if exp.tokType == TokenString && tok.Literal != exp.literal {
			t.Errorf("expected string literal %q, got %q", exp.literal, tok.Literal)
		}
	}
}

func TestLexerAsmBlock(t *testing.T) {
	input := `
	func wait_vblank() {
		asm {
		:   BIT $2002
			BPL :-
		}
	}
	`
	lexer := NewLexer("asm.m3", input)
	parser := NewParser(lexer)
	file, err := parser.ParseSourceFile()
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 func decl, got %d", len(file.Decls))
	}
	fn, ok := file.Decls[0].(*FuncDecl)
	if !ok {
		t.Fatalf("expected *FuncDecl")
	}

	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 stmt in body, got %d", len(fn.Body.Stmts))
	}

	asmStmt, ok := fn.Body.Stmts[0].(*AsmStmt)
	if !ok {
		t.Fatalf("expected *AsmStmt, got %T", fn.Body.Stmts[0])
	}

	if !stringsContains(asmStmt.Body, "BIT $2002") {
		t.Errorf("expected asm body to contain 'BIT $2002', got %q", asmStmt.Body)
	}
}

func TestLexerInclusionKeywords(t *testing.T) {
	input := `incbin incchr incpal`
	lexer := NewLexer("inc_test.m3", input)

	expected := []TokenType{TokenIncbin, TokenIncchr, TokenIncpal}
	for _, exp := range expected {
		tok := lexer.NextToken()
		if tok.Type != exp {
			t.Errorf("expected token type %v, got %v (%q)", exp, tok.Type, tok.Literal)
		}
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}()))
}

