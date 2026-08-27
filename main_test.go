package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleBuildDefaultOutput(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "testgame.m3")
	src := `
package main

func main() bank 0 {
    asm {
        NOP
    }
}
`
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if err := handleBuild([]string{srcFile}); err != nil {
		t.Fatalf("handleBuild failed: %v", err)
	}

	expectedOut := filepath.Join(tmpDir, "testgame.nes")
	stat, err := os.Stat(expectedOut)
	if err != nil {
		t.Fatalf("expected output %s was not created: %v", expectedOut, err)
	}
	if stat.Size() != 524304 {
		t.Errorf("expected file size 524304, got %d", stat.Size())
	}
}

func TestHandleBuildCustomOutputFlag(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "testgame.m3")
	outFile := filepath.Join(tmpDir, "flag_out.nes")
	src := `
package main

func main() bank 0 {
    asm {
        NOP
    }
}
`
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test -o
	if err := handleBuild([]string{srcFile, "-o", outFile}); err != nil {
		t.Fatalf("handleBuild with -o failed: %v", err)
	}

	stat, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("expected output %s was not created: %v", outFile, err)
	}
	if stat.Size() != 524304 {
		t.Errorf("expected file size 524304, got %d", stat.Size())
	}

	// Test --output
	outFile2 := filepath.Join(tmpDir, "flag_out2.nes")
	if err := handleBuild([]string{"--output", outFile2, srcFile}); err != nil {
		t.Fatalf("handleBuild with --output failed: %v", err)
	}
	stat2, err := os.Stat(outFile2)
	if err != nil {
		t.Fatalf("expected output %s was not created: %v", outFile2, err)
	}
	if stat2.Size() != 524304 {
		t.Errorf("expected file size 524304, got %d", stat2.Size())
	}
}

func TestHandleBuildRejectsNonM3Input(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "testgame.m3")
	outFile := filepath.Join(tmpDir, "output.nes")
	src := `
package main

func main() bank 0 {
    asm {
        NOP
    }
}
`
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Positional output.nes without -o should be treated as an input file and rejected because it is not .m3
	err := handleBuild([]string{srcFile, outFile})
	if err == nil {
		t.Fatal("expected error when passing non-.m3 positional argument without -o, got nil")
	}
	if !strings.Contains(err.Error(), "must be .m3") {
		t.Errorf("expected error message to mention .m3 files, got: %v", err)
	}
}

func TestHandleBuildMissingInput(t *testing.T) {
	if err := handleBuild([]string{}); err == nil {
		t.Fatal("expected error for empty arguments, got nil")
	}
	if err := handleBuild([]string{"-o", "out.nes"}); err == nil {
		t.Fatal("expected error for missing input files, got nil")
	}
}

func TestHandleBuildStandardLibraries(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "full_game.m3")
	outFile := filepath.Join(tmpDir, "full_game.nes")
	src := `
package main

import (
    "oam.m3"
    "ppu.m3"
    "memory.m3"
)

var (
    palette_data uint8[32] ram
    dest_buf     uint8[32] ram
)

func main() bank 0 {
    ppu.Disable()
    oam.Clear()
    oam.AdvanceFlicker()
    oam.PutSprite(100, 50, 1, 0)
    ppu.DirectUploadPalette(palette_data)
    memory.Copy(palette_data, dest_buf, 32)
    ppu.Enable()
}
`
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if err := handleBuild([]string{srcFile, "-o", outFile}); err != nil {
		t.Fatalf("handleBuild failed: %v", err)
	}

	stat, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("expected output %s was not created: %v", outFile, err)
	}
	if stat.Size() != 524304 {
		t.Errorf("expected file size 524304, got %d", stat.Size())
	}
}

func TestHandleBuildExamplesGame(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "game.nes")

	if err := handleBuild([]string{"examples/game.m3", "-o", outFile}); err != nil {
		t.Fatalf("handleBuild on examples/game.m3 failed: %v", err)
	}

	stat, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("expected output %s was not created: %v", outFile, err)
	}
	if stat.Size() != 524304 {
		t.Errorf("expected file size 524304, got %d", stat.Size())
	}
}

func TestHandleBuildDuplicateSymbolsAcrossPackages(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "pkgb")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	pkgBFile := filepath.Join(subDir, "b.m3")
	pkgBSrc := `
package pkgb

define (
    SHARED_REG $2000
    COUNTER_MAX 10
)

var scratch uint8 zp
var table uint8[8] ram

func Init() bank auto {
    scratch = 0
}
`
	if err := os.WriteFile(pkgBFile, []byte(pkgBSrc), 0644); err != nil {
		t.Fatalf("failed to write b.m3: %v", err)
	}

	mainFile := filepath.Join(tmpDir, "main.m3")
	mainSrc := `
package main

import "./pkgb/b.m3"

define (
    SHARED_REG $2000
    COUNTER_MAX 20
)

var scratch uint8 zp
var table uint8[8] ram

func main() bank 0 {
    scratch = 1
}
`
	if err := os.WriteFile(mainFile, []byte(mainSrc), 0644); err != nil {
		t.Fatalf("failed to write main.m3: %v", err)
	}

	outFile := filepath.Join(tmpDir, "dup_test.nes")
	if err := handleBuild([]string{mainFile, "-o", outFile}); err != nil {
		t.Fatalf("handleBuild failed with duplicate symbol conflict across packages: %v", err)
	}

	stat, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("expected output %s was not created: %v", outFile, err)
	}
	if stat.Size() != 524304 {
		t.Errorf("expected file size 524304, got %d", stat.Size())
	}
}

func TestHandleBuildDebugFlag(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "game.m3")
	src := `
package main

define (
    PPU_CTRL $2000
)

var player_x uint8 zp

func main() bank 0 {
    player_x = 5
}
`
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. Default output with -g
	if err := handleBuild([]string{srcFile, "-g"}); err != nil {
		t.Fatalf("handleBuild with -g failed: %v", err)
	}

	defaultNES := filepath.Join(tmpDir, "game.nes")
	defaultMLB := filepath.Join(tmpDir, "game.mlb")

	if _, err := os.Stat(defaultNES); err != nil {
		t.Fatalf("expected %s to be created: %v", defaultNES, err)
	}
	mlbBytes, err := os.ReadFile(defaultMLB)
	if err != nil {
		t.Fatalf("expected %s to be created: %v", defaultMLB, err)
	}
	mlbContent := string(mlbBytes)
	if !strings.Contains(mlbContent, "G:2000:_main_PPU_CTRL\n") {
		t.Errorf("expected G:2000:_main_PPU_CTRL in %s", defaultMLB)
	}
	if !strings.Contains(mlbContent, "R:0:_main_player_x\n") {
		t.Errorf("expected R:0:_main_player_x in %s", defaultMLB)
	}
	if !strings.Contains(mlbContent, "P:0:_main_main\n") {
		t.Errorf("expected P:0:_main_main in %s", defaultMLB)
	}

	// 2. Custom output with --debug
	customNES := filepath.Join(tmpDir, "custom.nes")
	customMLB := filepath.Join(tmpDir, "custom.mlb")
	if err := handleBuild([]string{"--debug", srcFile, "-o", customNES}); err != nil {
		t.Fatalf("handleBuild with --debug and -o failed: %v", err)
	}
	if _, err := os.Stat(customNES); err != nil {
		t.Fatalf("expected %s to be created: %v", customNES, err)
	}
	if _, err := os.Stat(customMLB); err != nil {
		t.Fatalf("expected %s to be created: %v", customMLB, err)
	}

	// 3. -g followed by -g0 disables debug symbol generation
	noDbgNES := filepath.Join(tmpDir, "nodbg.nes")
	noDbgMLB := filepath.Join(tmpDir, "nodbg.mlb")
	if err := handleBuild([]string{srcFile, "-o", noDbgNES, "-g", "-g0"}); err != nil {
		t.Fatalf("handleBuild with -g -g0 failed: %v", err)
	}
	if _, err := os.Stat(noDbgNES); err != nil {
		t.Fatalf("expected %s to be created: %v", noDbgNES, err)
	}
	if _, err := os.Stat(noDbgMLB); !os.IsNotExist(err) {
		t.Errorf("expected %s NOT to be created when -g0 is passed", noDbgMLB)
	}
}

func TestHandleLinkDebugFlag(t *testing.T) {
	tmpDir := t.TempDir()
	src := `
.export main, player_y
.zp
player_y: .res 1
.bank 0
.code
main:
    LDA #$02
    STA player_y
    RTS
`
	asmFile := filepath.Join(tmpDir, "linktest.s")
	moFile := filepath.Join(tmpDir, "linktest.mo")
	nesFile := filepath.Join(tmpDir, "linktest.nes")
	mlbFile := filepath.Join(tmpDir, "linktest.mlb")

	if err := os.WriteFile(asmFile, []byte(src), 0644); err != nil {
		t.Fatalf("failed to write asm file: %v", err)
	}

	if err := handleAssemble([]string{asmFile, moFile}); err != nil {
		t.Fatalf("handleAssemble failed: %v", err)
	}

	if err := handleLink([]string{moFile, "-o", nesFile, "-g"}); err != nil {
		t.Fatalf("handleLink with -g failed: %v", err)
	}

	if _, err := os.Stat(nesFile); err != nil {
		t.Fatalf("expected %s to be created", nesFile)
	}
	mlbBytes, err := os.ReadFile(mlbFile)
	if err != nil {
		t.Fatalf("expected %s to be created: %v", mlbFile, err)
	}
	mlbContent := string(mlbBytes)
	if !strings.Contains(mlbContent, "R:0:player_y\n") {
		t.Errorf("expected R:0:player_y in %s, got:\n%s", mlbFile, mlbContent)
	}
	if !strings.Contains(mlbContent, "P:0:main\n") {
		t.Errorf("expected P:0:main in %s, got:\n%s", mlbFile, mlbContent)
	}
}



