package compiler

import (
	"testing"
)

func TestParseCompleteLanguageExample(t *testing.T) {
	// Full example program from docs/language.md Section 11
	src := `
package main

// Hardware Registers
const (
    PPU_CTRL uint16 = $2000
    PPU_MASK uint16 = $2001
    PPU_STAT uint16 = $2002
)

// Striped Struct Definition
type Enemy struct {
    x      uint8
    y      uint8
    hp     uint8
    active bool
}

// Memory Allocations
var (
    frame_counter uint16   zp
    player_x      uint8    zp
    player_y      uint8    zp
    enemies       Enemy[8] ram
    high_score    uint32   wram
)

// PRG-ROM Data Table (Auto placed by Linker)
const enemy_spawn_x uint8[8] = [8]uint8{16, 48, 80, 112, 144, 176, 208, 240}

// Initialize Enemies
func init_enemies() {
    for i := uint8(0); i < 8; i++ {
        enemies[i].x = enemy_spawn_x[i]
        enemies[i].y = 32
        enemies[i].hp = 5
        enemies[i].active = true
    }
}

// Update Enemy Logic
func update_enemies() {
    for i := uint8(0); i < 8; i++ {
        if enemies[i].active {
            enemies[i].y++
            if enemies[i].y > 220 {
                enemies[i].y = 32
            }
        }
    }
}

// Main Game Entry Point
func main() bank 0 {
    player_x = 120
    player_y = 180
    init_enemies()

    for {
        // Wait for VBlank
        asm {
        :   BIT $2002
            BPL :-
        }

        frame_counter++
        update_enemies()
    }
}
`

	lexer := NewLexer("main.m3", src)
	parser := NewParser(lexer)

	file, err := parser.ParseSourceFile()
	if err != nil {
		t.Fatalf("failed to parse complete example program: %v", err)
	}

	if file.PackageName != "main" {
		t.Errorf("expected package name 'main', got %q", file.PackageName)
	}

	// Decls count:
	// 3 consts (from grouped const)
	// 1 type (Enemy)
	// 5 vars (from grouped var)
	// 1 const (enemy_spawn_x)
	// 3 funcs (init_enemies, update_enemies, main)
	// Total = 13 decls
	if len(file.Decls) != 13 {
		t.Errorf("expected 13 decls, got %d", len(file.Decls))
	}
}

func TestParseImports(t *testing.T) {
	src := `
	import "math/vector.m3"
	import (
		"types.m3"
		"audio/driver.m3"
	)
	`
	lexer := NewLexer("import.m3", src)
	parser := NewParser(lexer)
	file, err := parser.ParseSourceFile()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(file.Imports) != 3 {
		t.Fatalf("expected 3 imports, got %d", len(file.Imports))
	}
	if file.Imports[0].Path != "math/vector.m3" {
		t.Errorf("expected 'math/vector.m3', got %q", file.Imports[0].Path)
	}
	if file.Imports[1].Path != "types.m3" {
		t.Errorf("expected 'types.m3', got %q", file.Imports[1].Path)
	}
	if file.Imports[2].Path != "audio/driver.m3" {
		t.Errorf("expected 'audio/driver.m3', got %q", file.Imports[2].Path)
	}
}

func TestParseVariablesAndStorage(t *testing.T) {
	src := `
	var p_x uint8 zp
	var p_y uint8 zeropage
	var score uint32
	var table uint8[16] ram
	var buffer uint8[256] wram
	var work_buf uint8[1024] workram
	var ptr *uint8 zp
	`
	lexer := NewLexer("vars.m3", src)
	parser := NewParser(lexer)
	file, err := parser.ParseSourceFile()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(file.Decls) != 7 {
		t.Fatalf("expected 7 var decls, got %d", len(file.Decls))
	}

	v0 := file.Decls[0].(*VarDecl)
	if v0.Name != "p_x" || v0.Storage != StorageZP {
		t.Errorf("expected p_x in zp, got %v in %v", v0.Name, v0.Storage)
	}

	v1 := file.Decls[1].(*VarDecl)
	if v1.Name != "p_y" || v1.Storage != StorageZP {
		t.Errorf("expected p_y in zp, got %v in %v", v1.Name, v1.Storage)
	}

	v2 := file.Decls[2].(*VarDecl)
	if v2.Name != "score" || v2.Storage != StorageRAM {
		t.Errorf("expected score in ram, got %v in %v", v2.Name, v2.Storage)
	}

	v4 := file.Decls[4].(*VarDecl)
	if v4.Name != "buffer" || v4.Storage != StorageWRAM {
		t.Errorf("expected buffer in wram, got %v in %v", v4.Name, v4.Storage)
	}

	v6 := file.Decls[6].(*VarDecl)
	if _, ok := v6.Type.(*PointerType); !ok {
		t.Errorf("expected pointer type for ptr, got %T", v6.Type)
	}
}

func TestParseConstAndBanking(t *testing.T) {
	src := `
	const MAX uint8 = 10
	const table uint8[] bank auto = [2]uint8{1, 2}
	const palette uint8[16] bank 0 = [4]uint8{0, 1, 2, 3}
	const title string[] bank 63 = "SUPER GAME\0"
	`
	lexer := NewLexer("consts.m3", src)
	parser := NewParser(lexer)
	file, err := parser.ParseSourceFile()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(file.Decls) != 4 {
		t.Fatalf("expected 4 const decls, got %d", len(file.Decls))
	}

	c1 := file.Decls[1].(*ConstDecl)
	if c1.Bank == nil || !c1.Bank.IsAuto {
		t.Errorf("expected bank auto for c1")
	}

	c2 := file.Decls[2].(*ConstDecl)
	if c2.Bank == nil || c2.Bank.IsAuto {
		t.Errorf("expected fixed bank for c2")
	}
	if num, ok := c2.Bank.Value.(*NumberLit); !ok || num.Value != 0 {
		t.Errorf("expected bank 0 for c2, got %v", c2.Bank.Value)
	}
}

func TestParseControlFlow(t *testing.T) {
	src := `
	func test_flow(val uint8) uint8 {
		if val > 10 {
			val = 10
		} else if val < 2 {
			val = 2
		} else {
			val += 1
		}

		for val > 0 {
			val--
			if val == 5 {
				break
			} else {
				continue
			}
		}

		switch val {
		case 0, 1:
			return 10
		case 2:
			return 20
		default:
			return 0
		}
	}
	`
	lexer := NewLexer("flow.m3", src)
	parser := NewParser(lexer)
	file, err := parser.ParseSourceFile()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 func decl, got %d", len(file.Decls))
	}

	fn := file.Decls[0].(*FuncDecl)
	if len(fn.Params) != 1 || fn.Params[0].Name != "val" {
		t.Errorf("expected 1 param 'val'")
	}
	if fn.ReturnType == nil || fn.ReturnType.String() != "uint8" {
		t.Errorf("expected return type uint8")
	}

	stmts := fn.Body.Stmts
	if len(stmts) != 3 {
		t.Fatalf("expected 3 top statements in func body, got %d", len(stmts))
	}

	if _, ok := stmts[0].(*IfStmt); !ok {
		t.Errorf("expected IfStmt at 0, got %T", stmts[0])
	}
	if _, ok := stmts[1].(*ForStmt); !ok {
		t.Errorf("expected ForStmt at 1, got %T", stmts[1])
	}
	if _, ok := stmts[2].(*SwitchStmt); !ok {
		t.Errorf("expected SwitchStmt at 2, got %T", stmts[2])
	}
}

func TestParseUnaryAndBuiltins(t *testing.T) {
	src := `
	func test_expr() {
		var a uint8 zp
		a = <addr
		a = >addr
		a = ^symbol
		a = low(addr)
		a = high(addr)
		a = bank(symbol)
		a = uint8(10)
	}
	`
	lexer := NewLexer("expr.m3", src)
	parser := NewParser(lexer)
	file, err := parser.ParseSourceFile()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	fn := file.Decls[0].(*FuncDecl)
	if len(fn.Body.Stmts) != 8 {
		t.Fatalf("expected 8 statements, got %d", len(fn.Body.Stmts))
	}
}

func TestParseDefineDecl(t *testing.T) {
	src := `
	package main

	define PPU_CTRL $2000
	define PPU_MASK = $2001
	define (
		PPU_STAT $2002
		MAX_LIVES 3
		SCREEN_WIDTH (128 * 2)
	)

	func foo() {
		define LOCAL_DEF 42
	}
	`
	lexer := NewLexer("define.m3", src)
	parser := NewParser(lexer)
	file, err := parser.ParseSourceFile()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// 5 top-level defines + 1 func
	if len(file.Decls) != 6 {
		t.Fatalf("expected 6 top-level decls, got %d", len(file.Decls))
	}

	d0, ok := file.Decls[0].(*DefineDecl)
	if !ok || d0.Name != "PPU_CTRL" {
		t.Errorf("expected define PPU_CTRL, got %+v", file.Decls[0])
	}
	d1, ok := file.Decls[1].(*DefineDecl)
	if !ok || d1.Name != "PPU_MASK" {
		t.Errorf("expected define PPU_MASK, got %+v", file.Decls[1])
	}
	d2, ok := file.Decls[2].(*DefineDecl)
	if !ok || d2.Name != "PPU_STAT" {
		t.Errorf("expected define PPU_STAT, got %+v", file.Decls[2])
	}
	d3, ok := file.Decls[3].(*DefineDecl)
	if !ok || d3.Name != "MAX_LIVES" {
		t.Errorf("expected define MAX_LIVES, got %+v", file.Decls[3])
	}
	d4, ok := file.Decls[4].(*DefineDecl)
	if !ok || d4.Name != "SCREEN_WIDTH" {
		t.Errorf("expected define SCREEN_WIDTH, got %+v", file.Decls[4])
	}

	fn := file.Decls[5].(*FuncDecl)
	if len(fn.Body.Stmts) != 1 {
		t.Fatalf("expected 1 statement in foo(), got %d", len(fn.Body.Stmts))
	}
	if _, ok := fn.Body.Stmts[0].(*DefineDeclStmt); !ok {
		t.Errorf("expected DefineDeclStmt in foo(), got %T", fn.Body.Stmts[0])
	}
}

func TestParsePointerAndGroupedParams(t *testing.T) {
	src := `
	package main

	func DirectUploadPalette(pal *uint8[32]) {
	}

	func Copy(src, dst *uint8[], len uint16) {
	}

	func DirectUpload(src *uint8[], ppu_dst, len uint16) {
	}
	`
	lexer := NewLexer("lib_sig.m3", src)
	parser := NewParser(lexer)
	file, err := parser.ParseSourceFile()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(file.Decls) != 3 {
		t.Fatalf("expected 3 decls, got %d", len(file.Decls))
	}

	f1, ok := file.Decls[0].(*FuncDecl)
	if !ok || f1.Name != "DirectUploadPalette" {
		t.Fatalf("expected FuncDecl DirectUploadPalette, got %+v", file.Decls[0])
	}
	if len(f1.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(f1.Params))
	}
	if f1.Params[0].Name != "pal" {
		t.Errorf("expected param name pal, got %s", f1.Params[0].Name)
	}
	ptrType, ok := f1.Params[0].Type.(*PointerType)
	if !ok {
		t.Fatalf("expected PointerType, got %T", f1.Params[0].Type)
	}
	arrType, ok := ptrType.Elem.(*ArrayType)
	if !ok {
		t.Fatalf("expected ArrayType inside PointerType, got %T", ptrType.Elem)
	}
	if named, ok := arrType.Elem.(*NamedType); !ok || named.Name != "uint8" {
		t.Errorf("expected named type uint8, got %+v", arrType.Elem)
	}

	f2, ok := file.Decls[1].(*FuncDecl)
	if !ok || f2.Name != "Copy" {
		t.Fatalf("expected FuncDecl Copy, got %+v", file.Decls[1])
	}
	if len(f2.Params) != 3 {
		t.Fatalf("expected 3 params, got %d", len(f2.Params))
	}
	if f2.Params[0].Name != "src" || f2.Params[1].Name != "dst" || f2.Params[2].Name != "len" {
		t.Errorf("expected params [src dst len], got [%s %s %s]", f2.Params[0].Name, f2.Params[1].Name, f2.Params[2].Name)
	}

	f3, ok := file.Decls[2].(*FuncDecl)
	if !ok || f3.Name != "DirectUpload" {
		t.Fatalf("expected FuncDecl DirectUpload, got %+v", file.Decls[2])
	}
	if len(f3.Params) != 3 {
		t.Fatalf("expected 3 params for DirectUpload, got %d", len(f3.Params))
	}
	if f3.Params[0].Name != "src" || f3.Params[1].Name != "ppu_dst" || f3.Params[2].Name != "len" {
		t.Errorf("expected params [src ppu_dst len], got [%s %s %s]", f3.Params[0].Name, f3.Params[1].Name, f3.Params[2].Name)
	}
}


