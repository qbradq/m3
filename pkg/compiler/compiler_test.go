package compiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilerEndToEnd(t *testing.T) {
	src := `
package main

var player_x uint8 zp
var score uint32 ram
const MAX_LIVES uint8 = 3

func init_game() bank auto {
	player_x = 100
}

func main() bank 0 {
	asm {
	:   BIT $2002
		BPL :-
	}
}
`

	file, asmOutput, err := Compile("game.m3", src)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	if file == nil {
		t.Fatalf("expected AST file, got nil")
	}

	if !strings.Contains(asmOutput, "_player_x:") {
		t.Errorf("expected assembly to contain '_player_x:', got:\n%s", asmOutput)
	}
	if !strings.Contains(asmOutput, ".proc _main") {
		t.Errorf("expected assembly to contain '.proc _main', got:\n%s", asmOutput)
	}
	if !strings.Contains(asmOutput, "BIT $2002") {
		t.Errorf("expected assembly to contain inline asm, got:\n%s", asmOutput)
	}
}

func TestCompileFile(t *testing.T) {
	tmpDir := t.TempDir()
	m3Path := filepath.Join(tmpDir, "test.m3")
	sOutPath := filepath.Join(tmpDir, "test.s")

	src := `
package main
var x uint8 zp
func main() bank 0 {
	asm {
		NOP
	}
}
`
	if err := os.WriteFile(m3Path, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write m3 file: %v", err)
	}

	if err := CompileFile(m3Path, sOutPath); err != nil {
		t.Fatalf("CompileFile failed: %v", err)
	}

	content, err := os.ReadFile(sOutPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if !strings.Contains(string(content), "_x:") {
		t.Errorf("output missing _x:, got:\n%s", string(content))
	}
}
