package asm

import (
	"testing"
)

func TestExpressionEvaluation(t *testing.T) {
	symbols := &MapSymbolResolver{
		Symbols: map[string]int64{
			"BASE":   0x8000,
			"OFFSET": 0x20,
			"FLAG":   1,
		},
		Banks: map[string]int{
			"BASE": 4,
		},
	}

	tests := []struct {
		expr string
		want int64
	}{
		{"10 + 20 * 2", 50},
		{"(10 + 20) * 2", 60},
		{"100 / 4 - 5", 20},
		{"17 % 5", 2},
		{"1 << 8", 256},
		{"256 >> 4", 16},
		{"$FF & $0F", 0x0F},
		{"$F0 | $0F", 0xFF},
		{"$AA ^ $FF", 0x55},
		{"<BASE", 0x00},
		{">BASE", 0x80},
		{"^BASE", 4},
		{"BASE + OFFSET", 0x8020},
		{"FLAG && 1", 1},
		{"FLAG || 0", 1},
		{"!FLAG", 0},
		{"~0", -1},
		{"10 == 10", 1},
		{"10 != 10", 0},
		{"5 < 10", 1},
		{"10 <= 10", 1},
		{"15 > 10", 1},
		{"10 >= 15", 0},
	}

	for _, tt := range tests {
		lexer := NewLexer("test.m3", tt.expr)
		parser, err := NewParser(lexer)
		if err != nil {
			t.Fatalf("parser creation error for %q: %v", tt.expr, err)
		}
		parsedExpr, err := parser.parseExpression()
		if err != nil {
			t.Fatalf("failed to parse %q: %v", tt.expr, err)
		}
		got, err := parsedExpr.Eval(symbols)
		if err != nil {
			t.Fatalf("failed to eval %q: %v", tt.expr, err)
		}
		if got != tt.want {
			t.Errorf("Eval(%q) = %d, want %d", tt.expr, got, tt.want)
		}
	}
}
