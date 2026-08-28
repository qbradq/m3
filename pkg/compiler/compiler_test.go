package compiler

import (
	"image"
	"image/color"
	"image/png"
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

	if !strings.Contains(asmOutput, "_main_player_x:") {
		t.Errorf("expected assembly to contain '_main_player_x:', got:\n%s", asmOutput)
	}
	if !strings.Contains(asmOutput, ".proc _main_main") {
		t.Errorf("expected assembly to contain '.proc _main_main', got:\n%s", asmOutput)
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

	if !strings.Contains(string(content), "_main_x:") {
		t.Errorf("output missing _main_x:, got:\n%s", string(content))
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
		".export _main_PPU_CTRL",
		".define _main_PPU_CTRL $2000",
		".export _main_PPU_MASK",
		".define _main_PPU_MASK $2001",
		".define _main_local_const 42",
		".export _main_PPU_STAT",
		".define _main_PPU_STAT $2002",
		".export _main_CALC_VAL",
		".define _main_CALC_VAL (_main_PPU_CTRL + 5)",
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
        JSR _oam_Clear
    }
}
`
	_, asmOutput, err := Compile("main.m3", src)
	if err != nil {
		t.Fatalf("compilation with standard library import failed: %v", err)
	}

	// Verify that compile-time definitions from oam.m3 were imported
	expectedSnippets := []string{
		".define _oam_OAM_BUFFER $0200",
		".define _oam_SPR_PAL0 0",
		".proc _main_main",
		"JSR _oam_Clear",
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
        JSR _custom_oam_LocalHelper
    }
}
`
	gamePath := filepath.Join(subDir, "game.m3")
	_, asmOutput, err := Compile(gamePath, gameSrc)
	if err != nil {
		t.Fatalf("compilation with relative import failed: %v", err)
	}

	if !strings.Contains(asmOutput, "_custom_oam_LOCAL_SPR_COUNT") {
		t.Errorf("expected local oam.m3 definition _custom_oam_LOCAL_SPR_COUNT, got:\n%s", asmOutput)
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
        JSR _b_FuncB
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
    asm { JSR _d_HelperD }
}
`
	cSrc := `
package c
import "./d.m3"
func HelperC() bank auto {
    asm { JSR _d_HelperD }
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
        JSR _b_HelperB
        JSR _c_HelperC
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
        JSR _ppu_Disable
        LDA #<_main_my_palette
        LDX #>_main_my_palette
        JSR _ppu_DirectUploadPalette
        JSR _ppu_Enable
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
    memory.Copy(buf_src, buf_dst, 64)
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

func TestCompileInclusionExpressions(t *testing.T) {
	src := `
package main

const (
    font_pal   uint8[16] = incpal("font.png", 16)
    bg_pal     uint8[4]  = incpal("title.png")
    font_chr   uint8[]   = incchr("font.png")
    raw_data   uint8[]   = incbin("data.bin")
)

var fontPal uint8[16] = incpal("font.png", 16)

func main() bank 0 {
    asm {
        NOP
    }
}
`

	_, asmOutput, err := Compile("test_inc.m3", src)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	expectedDirectives := []string{
		".incpal \"font.png\", 16",
		".incpal \"title.png\"",
		".incchr \"font.png\"",
		".incbin \"data.bin\"",
		"_main_fontPal:\n  .res 16",
	}

	for _, exp := range expectedDirectives {
		if !strings.Contains(asmOutput, exp) {
			t.Errorf("expected assembly to contain %q, got:\n%s", exp, asmOutput)
		}
	}
}

func TestBuildInclusionExpressions(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create a valid test PAL and PNG (8x8 pixels, 4 colors)
	palText := "0:\n$0F\n$06\n$0A\n$02\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "font.pal"), []byte(palText), 0644); err != nil {
		t.Fatalf("failed to write temp font.pal: %v", err)
	}

	pal := color.Palette{
		color.RGBA{0, 0, 0, 0},         // $0F
		color.RGBA{84, 4, 0, 255},      // $06
		color.RGBA{73, 170, 16, 255},   // $0A
		color.RGBA{20, 18, 167, 255},   // $02
	}
	img := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	for x := 0; x < 8; x++ {
		img.SetColorIndex(x, 0, uint8(x%4))
	}
	pngPath := filepath.Join(tmpDir, "font.png")
	f, err := os.Create(pngPath)
	if err != nil {
		t.Fatalf("failed to create temp font.png: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("failed to encode temp font.png: %v", err)
	}
	f.Close()

	// 2. Create raw binary file
	binData := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	if err := os.WriteFile(filepath.Join(tmpDir, "raw.bin"), binData, 0644); err != nil {
		t.Fatalf("failed to write temp raw.bin: %v", err)
	}

	// 3. Create main.m3 using incpal, incchr, incbin
	mainSrc := `
package main

const (
    font_pal uint8[]   = incpal("font.pal")
    font_chr uint8[]   = incchr("font.png")
    bin_data uint8[]   = incbin("raw.bin")
)

var font_buf uint8[16] = incpal("font.pal")

func main() bank 0 {
    asm {
        LDA _main_font_pal
        LDA _main_bin_data
    }
}
`
	mainPath := filepath.Join(tmpDir, "main.m3")
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatalf("failed to write main.m3: %v", err)
	}

	rom, err := Build([]string{mainPath})
	if err != nil {
		t.Fatalf("Build with inclusion expressions failed: %v", err)
	}

	if len(rom) != 16+64*8192 {
		t.Fatalf("expected ROM size %d, got %d", 16+64*8192, len(rom))
	}
}

func TestCompileDataDeclarations(t *testing.T) {
	src := `
package testpkg

data title_image uint8[] bank 2 = incbin("title.bin")

data (
    FontPal uint8[16] = incpal("font.png", 16)
    FontChr uint8[] bank 3 = incchr("font.png")
    CustomTable [4]uint8 = [4]uint8{1, 2, 3, 4}
)
`
	_, asmOutput, err := Compile("test.m3", src)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expectedSnippets := []string{
		".bank 2",
		".data",
		"_testpkg_title_image:",
		".incbin \"title.bin\"",
		".bank auto",
		"_testpkg_FontPal:",
		".export _testpkg_FontPal",
		".incpal \"font.png\", 16",
		".bank 3",
		"_testpkg_FontChr:",
		".export _testpkg_FontChr",
		".incchr \"font.png\"",
		"_testpkg_CustomTable:",
		".export _testpkg_CustomTable",
		".byte $01, $02, $03, $04",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(asmOutput, snippet) {
			t.Errorf("expected assembly output to contain %q, got:\n%s", snippet, asmOutput)
		}
	}
}

func TestBuildMMCLibrary(t *testing.T) {
	tmpDir := t.TempDir()
	src := `
package main

import (
    "mmc.m3"
)

data level_data bank 5 = [4]uint8{10, 20, 30, 40}

func main() bank 0 {
    mmc.PushDataBank(^level_data)
    mmc.PopDataBank()
}
`
	mainPath := filepath.Join(tmpDir, "main.m3")
	if err := os.WriteFile(mainPath, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write main.m3: %v", err)
	}

	rom, err := Build([]string{mainPath})
	if err != nil {
		t.Fatalf("Build with mmc.m3 failed: %v", err)
	}

	if len(rom) != 16+64*8192 {
		t.Fatalf("expected ROM size %d, got %d", 16+64*8192, len(rom))
	}
}

func TestCompileStatements(t *testing.T) {
	src := `
package testgame

type Player struct {
    x  uint8
    y  uint8
    hp uint8
}

var (
    players Player[4] ram
    score   uint16    zp
    lives   uint8     zp
)

func update() {
    lives = 3
    score += 100
    lives--
    score++

    for i := uint8(0); i < 4; i++ {
        if players[i].hp > 0 {
            players[i].x += 2
            players[i].y++
        } else {
            players[i].x = 0
        }
    }

    switch lives {
    case 0:
        score = 0
    case 1:
        score = 10
    default:
        score = 100
    }
}
`
	_, asmOutput, err := Compile("testgame.m3", src)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expectedSnippets := []string{
		"_testgame_players_x:",
		".res 4",
		"_testgame_players_y:",
		".res 4",
		"_testgame_players_hp:",
		".res 4",
		"_testgame_update_i:",
		".res 1",
		".proc _testgame_update",
		"LDA #3",
		"STA _testgame_lives",
		"INC _testgame_score",
		"DEC _testgame_lives",
		"STA _testgame_update_i",
		"CMP #4",
		"STA _testgame_players_x, X",
		"INC _testgame_players_y, X",
		"JMP @for_head_",
		"CMP #0",
		"BEQ @case_",
		"RTS",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(asmOutput, snippet) {
			t.Errorf("expected assembly to contain %q, got:\n%s", snippet, asmOutput)
		}
	}
}

func TestBuildWithOptionsDebug(t *testing.T) {
	tmpDir := t.TempDir()
	src := `
package main

define (
    PPU_CTRL $2000
)

var (
    player_x uint8 zp
    score    uint16 ram
)

func main() bank 0 {
    player_x = 10
}
`
	mainPath := filepath.Join(tmpDir, "main.m3")
	if err := os.WriteFile(mainPath, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write main.m3: %v", err)
	}

	rom, symbols, err := BuildWithOptions([]string{mainPath}, BuildOptions{Debug: true})
	if err != nil {
		t.Fatalf("BuildWithOptions failed: %v", err)
	}
	if len(rom) != 16+64*8192 {
		t.Fatalf("unexpected ROM size: %d", len(rom))
	}
	if symbols == "" {
		t.Fatalf("expected non-empty symbols output")
	}

	if !strings.Contains(symbols, "G:2000:_main_PPU_CTRL\n") {
		t.Errorf("expected G:2000:_main_PPU_CTRL in symbols, got:\n%s", symbols)
	}
	if !strings.Contains(symbols, "R:0:_main_player_x\n") {
		t.Errorf("expected R:0:_main_player_x in symbols, got:\n%s", symbols)
	}
	if !strings.Contains(symbols, "R:300:_main_score\n") {
		t.Errorf("expected R:300:_main_score in symbols, got:\n%s", symbols)
	}
	if !strings.Contains(symbols, "P:0:_main_main\n") {
		t.Errorf("expected P:0:_main_main in symbols, got:\n%s", symbols)
	}
}

func TestBuildFilesWithOptionsDebug(t *testing.T) {
	tmpDir := t.TempDir()
	src := `
package main

var lives uint8 zp

func main() bank 0 {
    lives = 3
}
`
	mainPath := filepath.Join(tmpDir, "game.m3")
	if err := os.WriteFile(mainPath, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write game.m3: %v", err)
	}

	outNES := filepath.Join(tmpDir, "game.nes")
	outMLB := filepath.Join(tmpDir, "game.mlb")

	if err := BuildFilesWithOptions([]string{mainPath}, outNES, BuildOptions{Debug: true}); err != nil {
		t.Fatalf("BuildFilesWithOptions failed: %v", err)
	}

	if stat, err := os.Stat(outNES); err != nil || stat.Size() != 16+64*8192 {
		t.Fatalf("invalid NES ROM output: %v", err)
	}

	mlbBytes, err := os.ReadFile(outMLB)
	if err != nil {
		t.Fatalf("failed to read generated .mlb: %v", err)
	}
	mlbStr := string(mlbBytes)
	if !strings.Contains(mlbStr, "R:0:_main_lives\n") {
		t.Errorf("expected R:0:_main_lives in .mlb, got:\n%s", mlbStr)
	}
	if !strings.Contains(mlbStr, "P:0:_main_main\n") {
		t.Errorf("expected P:0:_main_main in .mlb, got:\n%s", mlbStr)
	}
}

func TestTopLevelBankEmission(t *testing.T) {
	src := `
package testpkg

bank 63

const TableA uint8[2] = [2]uint8{1, 2}

data DataA = incbin("sample.bin")

func FuncA() {
}

func FuncB() bank 2 {
}

bank 1

const TableB uint8[2] = [2]uint8{3, 4}

func FuncC() {
}
`
	_, asmOutput, err := Compile("test.m3", src)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	expectedSnippets := []string{
		// TableA in Bank 63
		".bank 63\n.code\n.export _testpkg_TableA\n_testpkg_TableA:",
		// DataA in Bank 63
		".bank 63\n.data\n.export _testpkg_DataA\n_testpkg_DataA:",
		// FuncA in Bank 63
		".bank 63\n.code\n.export _testpkg_FuncA\n.proc _testpkg_FuncA",
		// FuncB in Bank 2
		".bank 2\n.code\n.export _testpkg_FuncB\n.proc _testpkg_FuncB",
		// TableB in Bank 1
		".bank 1\n.code\n.export _testpkg_TableB\n_testpkg_TableB:",
		// FuncC in Bank 1
		".bank 1\n.code\n.export _testpkg_FuncC\n.proc _testpkg_FuncC",
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(asmOutput, snippet) {
			t.Errorf("expected asm output to contain %q, got:\n%s", snippet, asmOutput)
		}
	}
}

func TestCallArgCountValidation_PPU_DirectUpload_TooFew(t *testing.T) {
	src := `
package main

import "ppu.m3"

func main() bank 0 {
    ppu.DirectUpload(nil, $0000)
}
`
	_, _, err := Compile("test.m3", src)
	if err == nil {
		t.Fatal("expected compilation error for too few arguments to ppu.DirectUpload, got nil")
	}
	if !strings.Contains(err.Error(), "too few arguments in call to ppu.DirectUpload (expected 3, got 2)") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCallArgCountValidation_PPU_DirectUpload_TooMany(t *testing.T) {
	src := `
package main

import "ppu.m3"

func main() bank 0 {
    ppu.DirectUpload(nil, $0000, 4096, 10)
}
`
	_, _, err := Compile("test.m3", src)
	if err == nil {
		t.Fatal("expected compilation error for too many arguments to ppu.DirectUpload, got nil")
	}
	if !strings.Contains(err.Error(), "too many arguments in call to ppu.DirectUpload (expected 3, got 4)") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCallArgCountValidation_LocalFunc(t *testing.T) {
	src := `
package main

func add(a, b uint8) {
}

func main() bank 0 {
    add(1)
}
`
	_, _, err := Compile("test.m3", src)
	if err == nil {
		t.Fatal("expected compilation error for too few arguments to add, got nil")
	}
	if !strings.Contains(err.Error(), "too few arguments in call to add (expected 2, got 1)") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCallArgCountValidation_TypeCast(t *testing.T) {
	src := `
package main

func main() bank 0 {
    _ = uint8(1, 2)
}
`
	_, _, err := Compile("test.m3", src)
	if err == nil {
		t.Fatal("expected compilation error for invalid type cast argument count, got nil")
	}
	if !strings.Contains(err.Error(), "type conversion to uint8 requires exactly 1 argument, got 2") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestBuildPPUDriverLibrary(t *testing.T) {
	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "main.m3")
	mainSrc := `
package main

import "ppu_driver.m3"

var (
    msg uint8[16] ram
    pal uint8[32] ram
)

func nmi() bank 63 {
    ppu_driver.Process()
}

func main() bank 0 {
    ppu_driver.Clear()
    ppu_driver.PushHorizontal(msg, $2000, 16)
    ppu_driver.PushVertical(msg, $2020, 8)
    ppu_driver.PushByte($42, $23C0)
    ppu_driver.PushPalette(pal)
}
`
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatalf("failed to write main.m3: %v", err)
	}

	rom, err := Build([]string{mainPath})
	if err != nil {
		t.Fatalf("Build with ppu_driver.m3 failed: %v", err)
	}

	if len(rom) != 16+64*8192 {
		t.Fatalf("expected ROM size %d, got %d", 16+64*8192, len(rom))
	}
}

func TestFunctionCallRightToLeftParameters(t *testing.T) {
	src := `
package main

import "oam.m3"

type Enemy struct {
    x uint8
    y uint8
}

var (
    enemies Enemy[8] ram
    i uint8 zp
)

func main() bank 0 {
    oam.PutSprite(enemies[i].x, enemies[i].y, 0, 0)
    oam.PutSprite(0, enemies[0].y, 0, 0)
    oam.PutSprite(enemies[i].x+8, enemies[i].y+8, 16, 0)
}
`
	_, asmOutput, err := Compile("main.m3", src)
	if err != nil {
		t.Fatalf("compilation failed: %v", err)
	}

	// Verify right-to-left evaluation order and use of _reg_x_shadow
	if !strings.Contains(asmOutput, "STA __leaf_param0") {
		t.Errorf("expected assembly to store 4th argument to __leaf_param0, got:\n%s", asmOutput)
	}
	if !strings.Contains(asmOutput, "STA _reg_x_shadow") {
		t.Errorf("expected assembly to use _reg_x_shadow for Arg 1, got:\n%s", asmOutput)
	}
	if !strings.Contains(asmOutput, "LDX _reg_x_shadow") {
		t.Errorf("expected assembly to restore Arg 1 from _reg_x_shadow into X, got:\n%s", asmOutput)
	}

	// Check the first call: oam.PutSprite(enemies[i].x, enemies[i].y, 0, 0)
	// Arg 3: LDA #0 -> STA __leaf_param0
	// Arg 2: LDY #0
	// Arg 1: LDX _main_i -> LDA _main_enemies_y, X -> STA _reg_x_shadow
	// Arg 0: LDX _main_i -> LDA _main_enemies_x, X
	// Restore: LDX _reg_x_shadow
	// Call: JSR _oam_PutSprite
	expectedCallSequence := []string{
		"STA __leaf_param0",
		"LDY #0",
		"LDA _main_enemies_y, X",
		"STA _reg_x_shadow",
		"LDA _main_enemies_x, X",
		"LDX _reg_x_shadow",
		"JSR _oam_PutSprite",
	}
	lastIdx := 0
	for _, seq := range expectedCallSequence {
		idx := strings.Index(asmOutput[lastIdx:], seq)
		if idx == -1 {
			t.Fatalf("expected snippet %q after previous snippet in assembly:\n%s", seq, asmOutput)
		}
		lastIdx += idx + len(seq)
	}
}

func TestReadOnlyFunctionParameters(t *testing.T) {
	srcAssign := `
package main
func test(x uint8) {
    x = 10
}
`
	_, _, err := Compile("test.m3", srcAssign)
	if err == nil || !strings.Contains(err.Error(), "cannot assign to parameter \"x\" (function parameters are read-only)") {
		t.Fatalf("expected read-only parameter assignment error, got: %v", err)
	}

	srcInc := `
package main
func test(x uint8) {
    x++
}
`
	_, _, err = Compile("test.m3", srcInc)
	if err == nil || !strings.Contains(err.Error(), "cannot assign to parameter \"x\" (function parameters are read-only)") {
		t.Fatalf("expected read-only parameter inc/dec error, got: %v", err)
	}
}

func TestLeafAndNonLeafParameterAllocation(t *testing.T) {
	src := `
package main

func leaf_func(a, b, c uint8, d uint8) {
}

func non_leaf_func(a, b, c uint8, d uint8) {
    leaf_func(a, b, c, d)
}
`
	_, asmOutput, err := Compile("test.m3", src)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Non-leaf excess parameter d must be allocated in .ram
	if !strings.Contains(asmOutput, "_main_non_leaf_func_d:\n  .res 1") {
		t.Errorf("expected non-leaf excess param in .ram, got:\n%s", asmOutput)
	}
	// Leaf function excess param d should NOT be allocated in .ram
	if strings.Contains(asmOutput, "_main_leaf_func_d:") {
		t.Errorf("leaf function should not have dedicated .ram variable, got:\n%s", asmOutput)
	}
}

func TestBuildGameWithSpriteCalls(t *testing.T) {
	gamePath := filepath.Join("..", "..", "examples", "game.m3")
	rom, err := Build([]string{gamePath})
	if err != nil {
		t.Fatalf("Build game.m3 failed: %v", err)
	}
	if len(rom) != 16+64*8192 {
		t.Fatalf("expected ROM size %d, got %d", 16+64*8192, len(rom))
	}
}


