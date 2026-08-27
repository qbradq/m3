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


