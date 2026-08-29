package asm

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/qbradq/m3/pkg/obj"
)

func TestAssembleInstructionsAndAddressingModes(t *testing.T) {
	src := `
    .export start, loop
    .import ext_func
    .importzp zp_var

    PPU_CTRL = $2000

start:
    CLC             ; Implied (18)
    SEC             ; Implied (38)
    LSR A           ; Accumulator (4A)
    LDA #$42        ; Immediate (A9 42)
    LDA #<ext_func  ; Immediate reloc low byte (A9 00)
    LDA #>ext_func  ; Immediate reloc high byte (A9 00)
    LDA #^ext_func  ; Immediate reloc bank byte (A9 00)
    LDA $10         ; ZeroPage (A5 10)
    LDA $10, X      ; ZeroPageX (B5 10)
    LDX $10, Y      ; ZeroPageY (B6 10)
    LDA $2000       ; Absolute (AD 00 20)
    LDA $2000, X    ; AbsoluteX (BD 00 20)
    LDA $2000, Y    ; AbsoluteY (B9 00 20)
    LDA ($10, X)    ; IndirectX (A1 10)
    LDA ($10), Y    ; IndirectY (B1 10)
    JMP ($2000)     ; Indirect (6C 00 20)
    JSR ext_func    ; Absolute reloc (20 00 00)

loop:
    BNE loop        ; Relative (-2 -> FE)
:   BIT $2002
    BPL :-          ; Relative anon backward
    BEQ :+          ; Relative anon forward
:   RTS             ; Implied (60)
`
	objFile, err := Assemble("main.m3", src)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	if len(objFile.Banks) != 1 {
		t.Fatalf("expected 1 bank, got %d", len(objFile.Banks))
	}
	bank0 := objFile.Banks[0]
	if bank0.BankIndex != 0 {
		t.Errorf("expected bank 0, got %d", bank0.BankIndex)
	}

	// Verify specific opcode bytes
	expectedLeadingBytes := []byte{
		0x18,       // CLC
		0x38,       // SEC
		0x4A,       // LSR A
		0xA9, 0x42, // LDA #$42
		0xA9, 0x00, // LDA #<ext_func (reloc)
		0xA9, 0x00, // LDA #>ext_func (reloc)
		0xA9, 0x00, // LDA #^ext_func (reloc)
		0xA5, 0x10, // LDA $10 (ZP)
		0xB5, 0x10, // LDA $10, X (ZPX)
		0xB6, 0x10, // LDX $10, Y (ZPY)
		0xAD, 0x00, 0x20, // LDA $2000
		0xBD, 0x00, 0x20, // LDA $2000, X
		0xB9, 0x00, 0x20, // LDA $2000, Y
		0xA1, 0x10, // LDA ($10, X)
		0xB1, 0x10, // LDA ($10), Y
		0x6C, 0x00, 0x20, // JMP ($2000)
		0x20, 0x00, 0x00, // JSR ext_func (reloc)
		0xD0, 0xFE, // BNE loop (disp -2)
	}

	if len(bank0.Data) < len(expectedLeadingBytes) {
		t.Fatalf("bank data too short: got %d bytes, want at least %d", len(bank0.Data), len(expectedLeadingBytes))
	}

	if !bytes.Equal(bank0.Data[:len(expectedLeadingBytes)], expectedLeadingBytes) {
		t.Errorf("machine code mismatch:\ngot  % X\nwant % X", bank0.Data[:len(expectedLeadingBytes)], expectedLeadingBytes)
	}

	// Verify symbols
	symbolMap := make(map[string]obj.Symbol)
	for _, sym := range objFile.Symbols {
		symbolMap[sym.Name] = sym
	}

	startSym, ok := symbolMap["start"]
	if !ok {
		t.Fatal("symbol 'start' not found")
	}
	if startSym.Scope != obj.ScopeExport {
		t.Errorf("symbol 'start' scope = %v, want EXPORT", startSym.Scope)
	}
	if startSym.Value != 0 {
		t.Errorf("symbol 'start' value = %d, want 0", startSym.Value)
	}

	extSym, ok := symbolMap["ext_func"]
	if !ok {
		t.Fatal("symbol 'ext_func' not found")
	}
	if extSym.Type != obj.SymbolTypeImport {
		t.Errorf("symbol 'ext_func' type = %v, want IMPORT", extSym.Type)
	}
}

func TestAssembleMultiBank(t *testing.T) {
	src := `
.bank 0
bank0_code:
    LDA #$00
    RTS

.bank 1
bank1_data:
    .byte $01, $02, $03, $04
    .asciiz "HELLO"
    .word bank0_code

.bank 63
reset_vector:
    SEI
    CLD
    JMP bank0_code
`
	objFile, err := Assemble("multibank.m3", src)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	if len(objFile.Banks) != 3 {
		t.Fatalf("expected 3 banks, got %d", len(objFile.Banks))
	}

	bankMap := make(map[uint32]*obj.BankChunk)
	for _, b := range objFile.Banks {
		bankMap[b.BankIndex] = b
	}

	if b0, ok := bankMap[0]; !ok || len(b0.Data) != 3 { // LDA #$00 (2) + RTS (1) = 3
		t.Errorf("bank 0 invalid: len=%d", len(b0.Data))
	}

	if b1, ok := bankMap[1]; !ok {
		t.Errorf("bank 1 not found")
	} else {
		// 4 bytes + "HELLO\0" (6 bytes) + word (2 bytes) = 12 bytes
		if len(b1.Data) != 12 {
			t.Errorf("bank 1 len = %d, want 12", len(b1.Data))
		}
	}

	if b63, ok := bankMap[63]; !ok {
		t.Errorf("bank 63 not found")
	} else {
		// SEI (1) + CLD (1) + JMP bank0_code (3) = 5 bytes
		if len(b63.Data) != 5 {
			t.Errorf("bank 63 len = %d, want 5", len(b63.Data))
		}
	}
}

func TestLocalLabels(t *testing.T) {
	src := `
func_a:
    LDA #$01
@loop:
    DEX
    BNE @loop
    RTS

func_b:
    LDA #$02
@loop:
    DEY
    BNE @loop
    RTS
`
	objFile, err := Assemble("locals.m3", src)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	symMap := make(map[string]obj.Symbol)
	for _, s := range objFile.Symbols {
		symMap[s.Name] = s
	}

	if _, ok := symMap["func_a@loop"]; !ok {
		t.Errorf("expected func_a@loop symbol")
	}
	if _, ok := symMap["func_b@loop"]; !ok {
		t.Errorf("expected func_b@loop symbol")
	}
}

func TestIncchrAndIncpal(t *testing.T) {
	// Create a temporary PNG file and companion PAL file in temp dir
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "tile.png")
	palPath := filepath.Join(tmpDir, "tile.pal")

	palText := "0:\n$0F\n$06\n$0A\n$30\n"
	if err := os.WriteFile(palPath, []byte(palText), 0644); err != nil {
		t.Fatalf("failed to write temp pal: %v", err)
	}

	pal := color.Palette{
		color.RGBA{0, 0, 0, 0},         // Transparent -> $0F
		color.RGBA{84, 4, 0, 255},      // Reddish -> $06
		color.RGBA{73, 170, 16, 255},   // Green -> $0A
		color.RGBA{255, 255, 255, 255}, // White -> $30
	}
	img := image.NewPaletted(image.Rect(0, 0, 8, 8), pal)
	for x := 0; x < 8; x++ {
		img.SetColorIndex(x, 0, uint8(x%4))
	}
	f, err := os.Create(pngPath)
	if err != nil {
		t.Fatalf("failed to create temp png: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("failed to encode temp png: %v", err)
	}
	f.Close()

	src := fmt.Sprintf(`
.bank 0
palette:
    .incpal "%s"

tiles:
    .incchr "%s"
`, palPath, pngPath)

	objFile, err := Assemble("test_gfx.m3", src)
	if err != nil {
		t.Fatalf("assembly with .incchr and .incpal failed: %v", err)
	}

	if len(objFile.Banks) != 1 {
		t.Fatalf("expected 1 bank, got %d", len(objFile.Banks))
	}

	bank0 := objFile.Banks[0]
	// 16 bytes for default palette + 16 bytes for 1 tile = 32 bytes
	if len(bank0.Data) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(bank0.Data))
	}

	// Verify palette header (first 4 bytes from tile.pal, followed by 12 zeros)
	if bank0.Data[0] != 0x0F || bank0.Data[1] != 0x06 || bank0.Data[2] != 0x0A || bank0.Data[3] != 0x30 {
		t.Errorf("unexpected palette bytes: %v", bank0.Data[:4])
	}
	for i := 4; i < 16; i++ {
		if bank0.Data[i] != 0 {
			t.Errorf("expected padded zero at index %d, got $%02X", i, bank0.Data[i])
		}
	}

	// Verify tile plane 0 row 0 (at offset 16)
	if bank0.Data[16] != 0x55 {
		t.Errorf("expected tile plane 0 row 0 to be 0x55, got 0x%02X", bank0.Data[16])
	}
	// Verify tile plane 1 row 0 (at offset 24)
	if bank0.Data[24] != 0x33 {
		t.Errorf("expected tile plane 1 row 0 to be 0x33, got 0x%02X", bank0.Data[24])
	}

	// Test explicit count of 8 bytes (4 bytes from file, 4 padded zeroes)
	src8 := fmt.Sprintf(`.bank 0
palette:
    .incpal "%s", 8
`, palPath)
	objFile8, err := Assemble("test_count.m3", src8)
	if err != nil {
		t.Fatalf("assembly with explicit count failed: %v", err)
	}
	if len(objFile8.Banks[0].Data) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(objFile8.Banks[0].Data))
	}
	for i := 4; i < 8; i++ {
		if objFile8.Banks[0].Data[i] != 0 {
			t.Errorf("expected zero pad at %d, got $%02X", i, objFile8.Banks[0].Data[i])
		}
	}
}

func TestAssembleHelloWorldExample(t *testing.T) {
	hwPath := filepath.Join("..", "..", "examples", "hello_world.s")
	content, err := os.ReadFile(hwPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", hwPath, err)
	}

	objFile, err := Assemble(hwPath, string(content))
	if err != nil {
		t.Fatalf("failed to assemble hello_world.s: %v", err)
	}

	if len(objFile.Banks) != 3 {
		t.Fatalf("expected 3 banks, got %d", len(objFile.Banks))
	}

	bankMap := make(map[uint32]*obj.BankChunk)
	for _, b := range objFile.Banks {
		bankMap[b.BankIndex] = b
	}

	if _, ok := bankMap[0]; !ok {
		t.Errorf("expected Bank 0 in object file")
	}
	if b1, ok := bankMap[1]; !ok || len(b1.Data) == 0 {
		t.Errorf("expected Bank 1 with asset data")
	}
	if _, ok := bankMap[63]; !ok {
		t.Errorf("expected Bank 63 in object file")
	}
}

func TestMemoryAllocationDirectives(t *testing.T) {
	src := `
.zp
player_x: .res 1
player_y: .res 1
ptr:      .zp 2

.ram
level_data: .res 128
enemy_buf:  .ram 64

.wram
save_data:  .res 256
high_score: .wram 32

.bank 0
start:
    LDA player_x   ; Should generate Zero Page mode: A5 00 (2 bytes)
    STA player_y   ; Should generate Zero Page mode: 85 01 (2 bytes)
    LDA level_data ; Should generate Absolute mode: AD 00 00 (reloc ADDR16)
    STA save_data  ; Should generate Absolute mode: 8D 00 00 (reloc ADDR16)
`
	objFile, err := Assemble("test_mem.m3", src)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	symMap := make(map[string]obj.Symbol)
	for _, s := range objFile.Symbols {
		symMap[s.Name] = s
	}

	if s, ok := symMap["player_x"]; !ok || s.Bank != obj.BankZP || s.Value != 0 {
		t.Errorf("player_x invalid: %+v", s)
	}
	if s, ok := symMap["player_y"]; !ok || s.Bank != obj.BankZP || s.Value != 1 {
		t.Errorf("player_y invalid: %+v", s)
	}
	if s, ok := symMap["ptr"]; !ok || s.Bank != obj.BankZP || s.Value != 2 {
		t.Errorf("ptr invalid: %+v", s)
	}
	if s, ok := symMap["level_data"]; !ok || s.Bank != obj.BankRAM || s.Value != 0 {
		t.Errorf("level_data invalid: %+v", s)
	}
	if s, ok := symMap["enemy_buf"]; !ok || s.Bank != obj.BankRAM || s.Value != 128 {
		t.Errorf("enemy_buf invalid: %+v", s)
	}
	if s, ok := symMap["save_data"]; !ok || s.Bank != obj.BankWRAM || s.Value != 0 {
		t.Errorf("save_data invalid: %+v", s)
	}
	if s, ok := symMap["high_score"]; !ok || s.Bank != obj.BankWRAM || s.Value != 256 {
		t.Errorf("high_score invalid: %+v", s)
	}

	// Verify Bank 0 instructions: LDA player_x (A5 00), STA player_y (85 01)
	bank0 := objFile.Banks[0]
	if len(bank0.Data) < 4 {
		t.Fatalf("bank 0 too short: %d bytes", len(bank0.Data))
	}
	if bank0.Data[0] != 0xA5 { // LDA ZeroPage
		t.Errorf("expected LDA ZeroPage (0xA5), got 0x%02X", bank0.Data[0])
	}
	if bank0.Data[2] != 0x85 { // STA ZeroPage
		t.Errorf("expected STA ZeroPage (0x85), got 0x%02X", bank0.Data[2])
	}
}

func TestAssembleAutoBank(t *testing.T) {
	src := `
    .export auto_func1, auto_func2, auto_data
.bank auto
auto_func1:
    LDA #$42
    RTS

.bank auto
auto_func2:
    JSR auto_func1
    RTS

auto_data:
    .byte $11, $22, $33
`
	objFile, err := Assemble("auto_test.s", src)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	if len(objFile.Banks) != 1 {
		t.Fatalf("expected 1 bank chunk, got %d", len(objFile.Banks))
	}
	if objFile.Banks[0].BankIndex != obj.BankAutoIndex {
		t.Fatalf("expected bank index to be BankAutoIndex, got %d", objFile.Banks[0].BankIndex)
	}

	symMap := make(map[string]obj.Symbol)
	for _, s := range objFile.Symbols {
		symMap[s.Name] = s
	}

	if s, ok := symMap["auto_func1"]; !ok || s.Bank != obj.BankAuto || s.Value != 0 {
		t.Errorf("auto_func1 invalid: %+v", s)
	}
	if s, ok := symMap["auto_func2"]; !ok || s.Bank != obj.BankAuto || s.Value != 3 {
		t.Errorf("auto_func2 invalid: %+v", s)
	}
	if s, ok := symMap["auto_data"]; !ok || s.Bank != obj.BankAuto || s.Value != 7 {
		t.Errorf("auto_data invalid: %+v", s)
	}
}

func TestAssembleAutoBankLocalBranches(t *testing.T) {
	src := `
.bank auto
loop:
    DEX
    BNE loop
    RTS
`
	objFile, err := Assemble("branch_test.s", src)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	if len(objFile.Banks) != 1 {
		t.Fatalf("expected 1 bank chunk, got %d", len(objFile.Banks))
	}

	data := objFile.Banks[0].Data
	// DEX = CA
	// BNE loop -> D0 FD (-3 bytes)
	// RTS = 60
	expected := []byte{0xCA, 0xD0, 0xFD, 0x60}
	if len(data) != len(expected) {
		t.Fatalf("expected %d bytes, got %d", len(expected), len(data))
	}
	for i, b := range expected {
		if data[i] != b {
			t.Errorf("byte %d: got 0x%02X, want 0x%02X", i, data[i], b)
		}
	}
}

func TestDefineDirective(t *testing.T) {
	src := `
    .export EXPORTED_VAL, EXPORTED_CALC
    .define CONST_DEC 42
    .define CONST_HEX $2000
    .define CONST_BIN %10101010
    .define CONST_CHAR 'Z'
    .define CONST_EXPR (CONST_DEC * 2) + 5
    .define CONST_WITH_EQ = $1234
    .define CONST_WITH_COMMA, $5678
    INFIX_CONST .define 99
    .def DEF_ALIAS 100

    .define EXPORTED_VAL $ABCD
    .define EXPORTED_CALC EXPORTED_VAL + 1

    .zp CONST_DEC

    .bank 0
main:
    LDA #CONST_DEC           ; Immediate (A9 2A)
    LDA #CONST_CHAR          ; Immediate (A9 5A)
    LDA #<CONST_HEX          ; Low byte (A9 00)
    LDA #>CONST_HEX          ; High byte (A9 20)
    LDA #CONST_BIN           ; Immediate (A9 AA)
    LDA CONST_DEC            ; Zero page (A5 2A)
    LDA CONST_HEX            ; Absolute (AD 00 20)
    LDX #CONST_EXPR          ; Immediate: 42*2 + 5 = 89 (A9 59)
    LDY #INFIX_CONST         ; Immediate (A9 63)
    LDA #DEF_ALIAS           ; Immediate (A9 64)

data_block:
    .byte CONST_DEC, CONST_CHAR, CONST_BIN
    .word CONST_WITH_EQ, CONST_WITH_COMMA
    .res INFIX_CONST - 97, $EE
`
	objFile, err := Assemble("test_define.s", src)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	symMap := make(map[string]obj.Symbol)
	for _, s := range objFile.Symbols {
		symMap[s.Name] = s
	}

	// Verify exported definitions in symbol table
	if s, ok := symMap["EXPORTED_VAL"]; !ok {
		t.Errorf("EXPORTED_VAL not found in symbols")
	} else {
		if s.Scope != obj.ScopeExport {
			t.Errorf("expected EXPORTED_VAL scope EXPORT, got %v", s.Scope)
		}
		if s.Type != obj.SymbolTypeConst {
			t.Errorf("expected EXPORTED_VAL type CONST, got %v", s.Type)
		}
		if s.Bank != obj.BankConst {
			t.Errorf("expected EXPORTED_VAL bank %d, got %d", obj.BankConst, s.Bank)
		}
		if s.Value != 0xABCD {
			t.Errorf("expected EXPORTED_VAL value 0xABCD, got 0x%X", s.Value)
		}
	}

	if s, ok := symMap["EXPORTED_CALC"]; !ok {
		t.Errorf("EXPORTED_CALC not found in symbols")
	} else {
		if s.Scope != obj.ScopeExport {
			t.Errorf("expected EXPORTED_CALC scope EXPORT, got %v", s.Scope)
		}
		if s.Type != obj.SymbolTypeConst {
			t.Errorf("expected EXPORTED_CALC type CONST, got %v", s.Type)
		}
		if s.Value != 0xABCE {
			t.Errorf("expected EXPORTED_CALC value 0xABCE, got 0x%X", s.Value)
		}
	}

	// Verify constant symbols defined internally
	if s, ok := symMap["CONST_DEC"]; !ok || s.Value != 42 || s.Type != obj.SymbolTypeConst {
		t.Errorf("CONST_DEC symbol invalid: %+v", s)
	}
	if s, ok := symMap["CONST_EXPR"]; !ok || s.Value != 89 || s.Type != obj.SymbolTypeConst {
		t.Errorf("CONST_EXPR symbol invalid: %+v", s)
	}
	if s, ok := symMap["INFIX_CONST"]; !ok || s.Value != 99 || s.Type != obj.SymbolTypeConst {
		t.Errorf("INFIX_CONST symbol invalid: %+v", s)
	}
	if s, ok := symMap["DEF_ALIAS"]; !ok || s.Value != 100 || s.Type != obj.SymbolTypeConst {
		t.Errorf("DEF_ALIAS symbol invalid: %+v", s)
	}

	// Verify emitted bank 0 bytes
	if len(objFile.Banks) != 1 {
		t.Fatalf("expected 1 bank chunk, got %d", len(objFile.Banks))
	}
	bank0 := objFile.Banks[0]

	expectedInstructions := []byte{
		0xA9, 42,         // LDA #CONST_DEC
		0xA9, 'Z',        // LDA #CONST_CHAR ('Z' = 0x5A)
		0xA9, 0x00,       // LDA #<CONST_HEX
		0xA9, 0x20,       // LDA #>CONST_HEX
		0xA9, 0xAA,       // LDA #CONST_BIN (%10101010 = 0xAA)
		0xA5, 42,         // LDA CONST_DEC (ZeroPage)
		0xAD, 0x00, 0x20, // LDA CONST_HEX (Absolute $2000)
		0xA2, 89,         // LDX #CONST_EXPR (42*2 + 5 = 89)
		0xA0, 99,         // LDY #INFIX_CONST (99)
		0xA9, 100,        // LDA #DEF_ALIAS (100)
	}

	expectedData := []byte{
		42, 'Z', 0xAA, // .byte CONST_DEC, CONST_CHAR, CONST_BIN
		0x34, 0x12, // .word CONST_WITH_EQ ($1234)
		0x78, 0x56, // .word CONST_WITH_COMMA ($5678)
		0xEE, 0xEE, // .res 99-97 (2 bytes) filled with $EE
	}

	expectedAll := append(expectedInstructions, expectedData...)
	if !bytes.Equal(bank0.Data, expectedAll) {
		t.Errorf("bank 0 byte mismatch:\ngot  % X\nwant % X", bank0.Data, expectedAll)
	}
}

func TestDefineErrorOnUndefined(t *testing.T) {
	src := `
.define BAD_CONST UNDEFINED_SYM + 1
LDA #BAD_CONST
`
	_, err := Assemble("err.s", src)
	if err == nil {
		t.Errorf("expected error on undefined symbol in .define, got nil")
	}
}

