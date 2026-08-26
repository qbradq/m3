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

func TestCompileDefine(t *testing.T) {
	src := `
package main

define PPU_CTRL $2000
define PPU_MASK $2001
define local_const 42
define (
    PPU_STAT $2002
    CALC_VAL (PPU_CTRL + 5)
)
`
	_, asmOutput, err := Compile("define_test.m3", src)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expectedSnippets := []string{
		"; Compile-time Definitions",
		".export _PPU_CTRL",
		".define _PPU_CTRL $2000",
		".export _PPU_MASK",
		".define _PPU_MASK $2001",
		".define _local_const 42",
		".export _PPU_STAT",
		".define _PPU_STAT $2002",
		".export _CALC_VAL",
		".define _CALC_VAL (_PPU_CTRL + 5)",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(asmOutput, snippet) {
			t.Errorf("expected assembly output to contain %q, got:\n%s", snippet, asmOutput)
		}
	}
}

func TestImportLibraryInclude(t *testing.T) {
	src := `
package main

import "oam.m3"

func main() bank 0 {
    asm {
        JSR _Clear
    }
}
`
	_, asmOutput, err := Compile("main.m3", src)
	if err != nil {
		t.Fatalf("compilation with standard library import failed: %v", err)
	}

	// Verify that compile-time definitions from oam.m3 were imported
	expectedSnippets := []string{
		".define _OAM_BUFFER $0200",
		".define _SPR_PAL0 0",
		".proc _main",
		"JSR _Clear",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(asmOutput, snippet) {
			t.Errorf("expected assembly to contain %q from imported oam.m3, got:\n%s", snippet, asmOutput)
		}
	}
}

func TestImportRelativeInclude(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "game")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// 1. Local custom oam.m3 in subDir
	localOAM := `
package custom_oam

define LOCAL_SPR_COUNT 16

func LocalHelper() {
    asm {
        NOP
    }
}
`
	localOAMPath := filepath.Join(subDir, "oam.m3")
	if err := os.WriteFile(localOAMPath, []byte(localOAM), 0644); err != nil {
		t.Fatalf("failed to write local oam.m3: %v", err)
	}

	// 2. game.m3 imports "./oam.m3" (relative path)
	gameSrc := `
package main

import "./oam.m3"

func main() bank 0 {
    asm {
        JSR _LocalHelper
    }
}
`
	gamePath := filepath.Join(subDir, "game.m3")
	_, asmOutput, err := Compile(gamePath, gameSrc)
	if err != nil {
		t.Fatalf("compilation with relative import failed: %v", err)
	}

	if !strings.Contains(asmOutput, "_LOCAL_SPR_COUNT") {
		t.Errorf("expected local oam.m3 definition _LOCAL_SPR_COUNT, got:\n%s", asmOutput)
	}

	// 3. Test relative import failure when file does not exist in local directory
	missingGameSrc := `
package main

import "./missing.m3"

func main() bank 0 {}
`
	_, _, err = Compile(gamePath, missingGameSrc)
	if err == nil {
		t.Fatal("expected error for missing relative import, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read relative import") {
		t.Errorf("expected missing relative import error message, got: %v", err)
	}
}

func TestBuildSingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "main.m3")
	mainSrc := `
package main

var counter uint8 zp

func main() bank 0 {
    counter++
}
`
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatalf("failed to write main.m3: %v", err)
	}

	rom, err := Build([]string{mainPath})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if len(rom) != 16+64*8192 {
		t.Fatalf("expected ROM size %d, got %d", 16+64*8192, len(rom))
	}
}

func TestBuildWithStandardLibrary(t *testing.T) {
	tmpDir := t.TempDir()
	gamePath := filepath.Join(tmpDir, "game.m3")
	gameSrc := `
package main

import "oam.m3"

func main() bank 0 {
    oam.Clear()
    oam.AdvanceFlicker()
    oam.PutSprite(10, 20, 1, 0)
}
`
	if err := os.WriteFile(gamePath, []byte(gameSrc), 0644); err != nil {
		t.Fatalf("failed to write game.m3: %v", err)
	}

	outNES := filepath.Join(tmpDir, "game.nes")
	if err := BuildFiles([]string{gamePath}, outNES); err != nil {
		t.Fatalf("BuildFiles failed: %v", err)
	}

	stat, err := os.Stat(outNES)
	if err != nil {
		t.Fatalf("output NES ROM does not exist: %v", err)
	}
	if stat.Size() != int64(16+64*8192) {
		t.Fatalf("expected NES ROM size %d, got %d", 16+64*8192, stat.Size())
	}
}

func TestBuildCyclicImports(t *testing.T) {
	tmpDir := t.TempDir()
	aPath := filepath.Join(tmpDir, "a.m3")
	bPath := filepath.Join(tmpDir, "b.m3")

	// a.m3 imports b.m3, b.m3 imports a.m3
	aSrc := `
package a

import "./b.m3"

define CONST_A 100

func FuncA() bank auto {
    asm {
        JSR _FuncB
    }
}

func main() bank 0 {
    FuncA()
}
`
	bSrc := `
package b

import "./a.m3"

define CONST_B 200

func FuncB() bank auto {
    asm {
        NOP
    }
}
`
	if err := os.WriteFile(aPath, []byte(aSrc), 0644); err != nil {
		t.Fatalf("failed to write a.m3: %v", err)
	}
	if err := os.WriteFile(bPath, []byte(bSrc), 0644); err != nil {
		t.Fatalf("failed to write b.m3: %v", err)
	}

	rom, err := Build([]string{aPath})
	if err != nil {
		t.Fatalf("Build with cyclic imports failed: %v", err)
	}

	if len(rom) != 16+64*8192 {
		t.Fatalf("expected ROM size %d, got %d", 16+64*8192, len(rom))
	}
}

func TestBuildDiamondImports(t *testing.T) {
	tmpDir := t.TempDir()
	aPath := filepath.Join(tmpDir, "a.m3")
	bPath := filepath.Join(tmpDir, "b.m3")
	cPath := filepath.Join(tmpDir, "c.m3")
	dPath := filepath.Join(tmpDir, "d.m3")

	// A imports B and C. Both B and C import D.
	dSrc := `
package d
define BASE_VAL 42
func HelperD() bank auto {
    asm { NOP }
}
`
	bSrc := `
package b
import "./d.m3"
func HelperB() bank auto {
    asm { JSR _HelperD }
}
`
	cSrc := `
package c
import "./d.m3"
func HelperC() bank auto {
    asm { JSR _HelperD }
}
`
	aSrc := `
package main
import (
    "./b.m3"
    "./c.m3"
)
func main() bank 0 {
    asm {
        JSR _HelperB
        JSR _HelperC
    }
}
`
	if err := os.WriteFile(dPath, []byte(dSrc), 0644); err != nil {
		t.Fatalf("failed to write d.m3: %v", err)
	}
	if err := os.WriteFile(bPath, []byte(bSrc), 0644); err != nil {
		t.Fatalf("failed to write b.m3: %v", err)
	}
	if err := os.WriteFile(cPath, []byte(cSrc), 0644); err != nil {
		t.Fatalf("failed to write c.m3: %v", err)
	}
	if err := os.WriteFile(aPath, []byte(aSrc), 0644); err != nil {
		t.Fatalf("failed to write a.m3: %v", err)
	}

	units, err := AccumulateSourceFiles([]string{aPath})
	if err != nil {
		t.Fatalf("AccumulateSourceFiles failed: %v", err)
	}

	// Should have exactly 4 unique files: a, b, d, c (or a, b, c, d)
	if len(units) != 4 {
		t.Fatalf("expected 4 accumulated units, got %d", len(units))
	}

	rom, err := Build([]string{aPath})
	if err != nil {
		t.Fatalf("Build with diamond imports failed: %v", err)
	}

	if len(rom) != 16+64*8192 {
		t.Fatalf("expected ROM size %d, got %d", 16+64*8192, len(rom))
	}
}

func TestBuildPPULibrary(t *testing.T) {
	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "main.m3")
	mainSrc := `
package main

import "ppu.m3"

const my_palette uint8[32] = [32]uint8{
    $0F, $00, $10, $30, $0F, $01, $11, $31,
    $0F, $02, $12, $32, $0F, $03, $13, $33,
    $0F, $04, $14, $34, $0F, $05, $15, $35,
    $0F, $06, $16, $36, $0F, $07, $17, $37,
}

func main() bank 0 {
    asm {
        JSR _Disable
        LDA #<my_palette
        LDX #>my_palette
        JSR _DirectUploadPalette
        JSR _Enable
    }
}
`
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatalf("failed to write main.m3: %v", err)
	}

	rom, err := Build([]string{mainPath})
	if err != nil {
		t.Fatalf("Build with ppu.m3 failed: %v", err)
	}

	if len(rom) != 16+64*8192 {
		t.Fatalf("expected ROM size %d, got %d", 16+64*8192, len(rom))
	}
}

func TestBuildMemoryLibrary(t *testing.T) {
	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "main.m3")
	mainSrc := `
package main

import "memory.m3"

var (
    buf_src uint8[64] ram
    buf_dst uint8[64] ram
)

func main() bank 0 {
    asm {
        LDA #<buf_src
        STA _src_ptr
        LDA #>buf_src
        STA _src_ptr+1

        LDA #<buf_dst
        STA _dst_ptr
        LDA #>buf_dst
        STA _dst_ptr+1

        LDA #64
        STA _len_cnt
        LDA #0
        STA _len_cnt+1

        JSR _Copy
    }
}
`
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatalf("failed to write main.m3: %v", err)
	}

	rom, err := Build([]string{mainPath})
	if err != nil {
		t.Fatalf("Build with memory.m3 failed: %v", err)
	}

	if len(rom) != 16+64*8192 {
		t.Fatalf("expected ROM size %d, got %d", 16+64*8192, len(rom))
	}
}


