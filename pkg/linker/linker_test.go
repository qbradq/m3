package linker

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qbradq/m3/pkg/asm"
	"github.com/qbradq/m3/pkg/compiler"
	"github.com/qbradq/m3/pkg/data"
)

func TestLinkSingleObject(t *testing.T) {
	src := `
    .export main, reset_handler, nmi_handler, irq_handler
    PPU_CTRL = $2000

.bank 0
main:
    LDA #$00
    STA PPU_CTRL
    JSR helper
    JMP main

helper:
    RTS

.bank 63
reset_handler:
    SEI
    CLD
    JMP main

nmi_handler:
    RTI

irq_handler:
    RTI
`
	objFile, err := asm.Assemble("main.m3", src)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	l := NewLinker(objFile)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("linking failed: %v", err)
	}

	if len(rom) != TotalOutputSize {
		t.Fatalf("expected ROM size %d, got %d", TotalOutputSize, len(rom))
	}

	// Verify iNES header
	if string(rom[0:4]) != "NES\x1A" {
		t.Errorf("invalid iNES header magic: %v", rom[0:4])
	}
	if rom[4] != 32 { // 512KB PRG = 32 * 16KB
		t.Errorf("expected 32 PRG 16KB units, got %d", rom[4])
	}
	if rom[5] != 0 { // 0 CHR ROM units (CHR-RAM)
		t.Errorf("expected 0 CHR units, got %d", rom[5])
	}
	if rom[6] != 0x40 { // Mapper 4 lower nibble (0x40)
		t.Errorf("expected flags6 = 0x40, got 0x%02X", rom[6])
	}

	// Verify Bank 0 machine code at offset 16 (Header) + 0 (Bank 0)
	bank0Offset := INESHeaderSize
	// main is at $8000: LDA #$00 (A9 00), STA $2000 (8D 00 20), JSR helper (20 0B 80), JMP main (4C 00 80), helper: RTS (60)
	expectedBank0 := []byte{
		0xA9, 0x00, // LDA #$00
		0x8D, 0x00, 0x20, // STA $2000
		0x20, 0x0B, 0x80, // JSR $800B (helper)
		0x4C, 0x00, 0x80, // JMP $8000 (main)
		0x60, // RTS
	}
	for i, exp := range expectedBank0 {
		if rom[bank0Offset+i] != exp {
			t.Errorf("bank 0 byte at %d mismatch: got 0x%02X, want 0x%02X", i, rom[bank0Offset+i], exp)
		}
	}

	// Verify Bank 63 vectors at offset: 16 + 63 * 8192 = 516112
	bank63Offset := INESHeaderSize + 63*BankSize
	nmiAddr := binary.LittleEndian.Uint16(rom[bank63Offset+NMIVectorOffset : bank63Offset+NMIVectorOffset+2])
	resetAddr := binary.LittleEndian.Uint16(rom[bank63Offset+ResetVectorOffset : bank63Offset+ResetVectorOffset+2])
	irqAddr := binary.LittleEndian.Uint16(rom[bank63Offset+IRQVectorOffset : bank63Offset+IRQVectorOffset+2])

	if resetAddr != 0xE000 {
		t.Errorf("reset vector: got $%04X, want $E000", resetAddr)
	}
	if nmiAddr != 0xE005 { // reset_handler is SEI (1) + CLD (1) + JMP main (3) = 5 bytes -> nmi_handler is at $E005
		t.Errorf("nmi vector: got $%04X, want $E005", nmiAddr)
	}
	if irqAddr != 0xE006 { // nmi_handler is RTI (1 byte) -> irq_handler is at $E006
		t.Errorf("irq vector: got $%04X, want $E006", irqAddr)
	}
}

func TestLinkMultipleObjects(t *testing.T) {
	srcA := `
    .export func_a
    .import func_b

.bank 0
func_a:
    LDA #$01
    JSR func_b
    RTS
`
	srcB := `
    .export func_b
    .import func_a

.bank 0
func_b:
    LDA #$02
    JSR func_a
    RTS
`
	srcC := `
    .export reset_handler
    .import func_a

.bank 63
reset_handler:
    SEI
    CLD
    JSR func_a
:   JMP :-
`
	objA, err := asm.Assemble("fileA.m3", srcA)
	if err != nil {
		t.Fatalf("failed to assemble fileA: %v", err)
	}
	objB, err := asm.Assemble("fileB.m3", srcB)
	if err != nil {
		t.Fatalf("failed to assemble fileB: %v", err)
	}
	objC, err := asm.Assemble("fileC.m3", srcC)
	if err != nil {
		t.Fatalf("failed to assemble fileC: %v", err)
	}

	l := NewLinker(objA, objB, objC)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link multiple objects: %v", err)
	}

	// func_a in Bank 0 starts at $8000: LDA #$01 (2), JSR func_b (3), RTS (1) = 6 bytes
	// func_b in Bank 0 starts at $8006: LDA #$02 (2), JSR func_a (3), RTS (1) = 6 bytes
	bank0Offset := INESHeaderSize
	expectedBank0 := []byte{
		0xA9, 0x01, // LDA #$01
		0x20, 0x06, 0x80, // JSR $8006 (func_b)
		0x60, // RTS
		0xA9, 0x02, // LDA #$02
		0x20, 0x00, 0x80, // JSR $8000 (func_a)
		0x60, // RTS
	}
	for i, exp := range expectedBank0 {
		if rom[bank0Offset+i] != exp {
			t.Errorf("byte %d in bank 0 mismatch: got 0x%02X, want 0x%02X", i, rom[bank0Offset+i], exp)
		}
	}

	// Check reset vector in bank 63
	bank63Offset := INESHeaderSize + 63*BankSize
	resetAddr := binary.LittleEndian.Uint16(rom[bank63Offset+ResetVectorOffset : bank63Offset+ResetVectorOffset+2])
	if resetAddr != 0xE000 {
		t.Errorf("reset vector = $%04X, want $E000", resetAddr)
	}
}

func TestDuplicateSymbolError(t *testing.T) {
	src1 := `
    .export dup_func
.bank 0
dup_func:
    RTS
`
	src2 := `
    .export dup_func
.bank 0
dup_func:
    NOP
    RTS
`
	obj1, _ := asm.Assemble("file1.m3", src1)
	obj2, _ := asm.Assemble("file2.m3", src2)

	l := NewLinker(obj1, obj2)
	_, err := l.Link()
	if err == nil {
		t.Fatal("expected duplicate symbol error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate symbol") {
		t.Errorf("expected duplicate symbol error message, got: %v", err)
	}
}

func TestUndefinedSymbolError(t *testing.T) {
	src := `
    .export reset_handler
    .import missing_func
.bank 63
reset_handler:
    JSR missing_func
    RTS
`
	objFile, _ := asm.Assemble("file.m3", src)
	l := NewLinker(objFile)
	_, err := l.Link()
	if err == nil {
		t.Fatal("expected undefined symbol error, got nil")
	}
	if !strings.Contains(err.Error(), "undefined symbol") {
		t.Errorf("expected undefined symbol error message, got: %v", err)
	}
}

func TestBankOverflowError(t *testing.T) {
	src := `
    .export reset_handler
.bank 0
large_data:
    .res 8193, $FF

.bank 63
reset_handler:
    RTS
`
	objFile, _ := asm.Assemble("overflow.m3", src)
	l := NewLinker(objFile)
	_, err := l.Link()
	if err == nil {
		t.Fatal("expected bank overflow error, got nil")
	}
	if !strings.Contains(err.Error(), "overflow") {
		t.Errorf("expected overflow error message, got: %v", err)
	}
}

func TestRelocationByteSelectors(t *testing.T) {
	src := `
    .export reset_handler, test_label
.bank 1
test_label:
    .byte $55

.bank 63
reset_handler:
    LDA #<test_label
    LDX #>test_label
    LDY #^test_label
    RTS
`
	objFile, err := asm.Assemble("relocs.m3", src)
	if err != nil {
		t.Fatalf("assembly failed: %v", err)
	}

	l := NewLinker(objFile)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("linking failed: %v", err)
	}

	bank63Offset := INESHeaderSize + 63*BankSize
	// test_label in Bank 1 is at offset 0 (address = 0)
	// LDA #<test_label -> A9 00
	// LDX #>test_label -> A2 00
	// LDY #^test_label -> A0 01 (Bank 1)
	// RTS              -> 60
	expectedBank63 := []byte{
		0xA9, 0x00, // LDA #<0 = 0
		0xA2, 0x00, // LDX #>0 = 0
		0xA0, 0x01, // LDY #^test_label = 1 (Bank 1)
		0x60, // RTS
	}
	for i, exp := range expectedBank63 {
		if rom[bank63Offset+i] != exp {
			t.Errorf("byte %d in bank 63 mismatch: got 0x%02X, want 0x%02X", i, rom[bank63Offset+i], exp)
		}
	}
}

func TestLinkHelloWorldExample(t *testing.T) {
	hwPath := filepath.Join("..", "..", "examples", "hello_world.s")
	content, err := os.ReadFile(hwPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", hwPath, err)
	}

	objFile, err := asm.Assemble(hwPath, string(content))
	if err != nil {
		t.Fatalf("failed to assemble hello_world.s: %v", err)
	}

	tmpDir := t.TempDir()
	nesPath := filepath.Join(tmpDir, "hello_world.nes")

	l := NewLinker(objFile)
	romData, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link hello_world: %v", err)
	}

	if err := os.WriteFile(nesPath, romData, 0644); err != nil {
		t.Fatalf("failed to write hello_world.nes: %v", err)
	}

	info, err := os.Stat(nesPath)
	if err != nil {
		t.Fatalf("failed to stat hello_world.nes: %v", err)
	}
	if info.Size() != int64(TotalOutputSize) {
		t.Fatalf("expected file size %d, got %d", TotalOutputSize, info.Size())
	}
}

func TestLinkRAMSegments(t *testing.T) {
	srcA := `
    .export px, py, ram_buf, wram_save
.zp
px: .res 1
py: .res 2

.ram
ram_buf: .res 64

.wram
wram_save: .res 128
`
	srcB := `
    .export ex, ey, main, reset_handler
    .importzp px, py
    .import ram_buf, wram_save
.zp
ex: .res 1
ey: .res 1

.bank 0
main:
    LDA px
    STA ex
    LDA ram_buf
    STA wram_save
    RTS

.bank 63
reset_handler:
    JMP main
`
	objA, err := asm.Assemble("fileA.m3", srcA)
	if err != nil {
		t.Fatalf("failed to assemble fileA: %v", err)
	}
	objB, err := asm.Assemble("fileB.m3", srcB)
	if err != nil {
		t.Fatalf("failed to assemble fileB: %v", err)
	}

	l := NewLinker(objA, objB)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link RAM segments: %v", err)
	}

	// Verify battery-backed flag is set in flags 6 (bit 1)
	if rom[6]&0x02 == 0 {
		t.Errorf("expected battery-backed flag (bit 1) to be set in flags6, got 0x%02X", rom[6])
	}

	// Verify Bank 0 machine code:
	// LDA px ($00)        -> A5 00
	// STA ex ($03)        -> 85 03 (fileA used 3 bytes: px(1) + py(2) = 3 bytes -> fileB starts at $03)
	// LDA ram_buf ($0300) -> AD 00 03
	// STA wram_save ($6000)-> 8D 00 60
	// RTS                 -> 60
	bank0Offset := INESHeaderSize
	expectedBank0 := []byte{
		0xA5, 0x00, // LDA $00 (px)
		0x85, 0x03, // STA $03 (ex)
		0xAD, 0x00, 0x03, // LDA $0300 (ram_buf)
		0x8D, 0x00, 0x60, // STA $6000 (wram_save)
		0x60, // RTS
	}
	for i, exp := range expectedBank0 {
		if rom[bank0Offset+i] != exp {
			t.Errorf("byte %d in bank 0 mismatch: got 0x%02X, want 0x%02X", i, rom[bank0Offset+i], exp)
		}
	}
}

func TestZPOverflow(t *testing.T) {
	src := `
    .export reset_handler
.zp
large_zp: .res 257
.bank 63
reset_handler:
    RTS
`
	objFile, err := asm.Assemble("zp_overflow.m3", src)
	if err != nil {
		t.Fatalf("failed to assemble: %v", err)
	}
	l := NewLinker(objFile)
	_, err = l.Link()
	if err == nil {
		t.Fatal("expected ZP overflow error, got nil")
	}
	if !strings.Contains(err.Error(), "zero page overflow") {
		t.Errorf("expected zero page overflow error message, got: %v", err)
	}
}

func TestRAMOverflow(t *testing.T) {
	src := `
    .export reset_handler
.ram
large_ram: .res 1281
.bank 63
reset_handler:
    RTS
`
	objFile, err := asm.Assemble("ram_overflow.m3", src)
	if err != nil {
		t.Fatalf("failed to assemble: %v", err)
	}
	l := NewLinker(objFile)
	_, err = l.Link()
	if err == nil {
		t.Fatal("expected RAM overflow error, got nil")
	}
	if !strings.Contains(err.Error(), "main RAM overflow") {
		t.Errorf("expected main RAM overflow error message, got: %v", err)
	}
}

func TestWRAMOverflow(t *testing.T) {
	src := `
    .export reset_handler
.wram
large_wram: .res 8193
.bank 63
reset_handler:
    RTS
`
	objFile, err := asm.Assemble("wram_overflow.m3", src)
	if err != nil {
		t.Fatalf("failed to assemble: %v", err)
	}
	l := NewLinker(objFile)
	_, err = l.Link()
	if err == nil {
		t.Fatal("expected WRAM overflow error, got nil")
	}
	if !strings.Contains(err.Error(), "work RAM overflow") {
		t.Errorf("expected work RAM overflow error message, got: %v", err)
	}
}

func TestLinkAutoBank(t *testing.T) {
	src := `
    .export reset_handler, auto_func
.bank auto
auto_func:
    LDA #$42
    RTS

.bank 63
reset_handler:
    JSR auto_func
    LDA #^auto_func
    RTS
`
	objFile, err := asm.Assemble("auto.s", src)
	if err != nil {
		t.Fatalf("failed to assemble auto.s: %v", err)
	}

	l := NewLinker(objFile)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link auto object: %v", err)
	}

	// auto_func should be placed in Bank 0 at $8000: LDA #$42 (A9 42), RTS (60)
	bank0Offset := INESHeaderSize
	expectedBank0 := []byte{0xA9, 0x42, 0x60}
	for i, exp := range expectedBank0 {
		if rom[bank0Offset+i] != exp {
			t.Errorf("byte %d in bank 0 mismatch: got 0x%02X, want 0x%02X", i, rom[bank0Offset+i], exp)
		}
	}

	// Bank 63 at reset_handler: JSR $8000 (20 00 80), LDA #^auto_func (A9 00), RTS (60)
	bank63Offset := INESHeaderSize + 63*BankSize
	expectedBank63 := []byte{0x20, 0x00, 0x80, 0xA9, 0x00, 0x60}
	for i, exp := range expectedBank63 {
		if rom[bank63Offset+i] != exp {
			t.Errorf("byte %d in bank 63 mismatch: got 0x%02X, want 0x%02X", i, rom[bank63Offset+i], exp)
		}
	}
}

func TestLinkMultipleAutoBanks(t *testing.T) {
	// File A fills Bank 0 almost completely (8190 bytes)
	srcA := `
.bank 0
fill_data:
    .res 8190, $EA
`
	// File B has auto bank of 100 bytes (cannot fit in Bank 0, should go to Bank 1)
	srcB := `
    .export auto_b
.bank auto
auto_b:
    .res 100, $BB
`
	// File C has auto bank of 50 bytes (fits into Bank 1 alongside File B)
	srcC := `
    .export auto_c
.bank auto
auto_c:
    .res 50, $CC
`
	// File D references auto_b and auto_c bank bytes
	srcD := `
    .export reset_handler
    .import auto_b, auto_c
.bank 63
reset_handler:
    LDA #^auto_b
    LDX #^auto_c
    RTS
`
	objA, err := asm.Assemble("fileA.s", srcA)
	if err != nil {
		t.Fatalf("failed to assemble fileA: %v", err)
	}
	objB, err := asm.Assemble("fileB.s", srcB)
	if err != nil {
		t.Fatalf("failed to assemble fileB: %v", err)
	}
	objC, err := asm.Assemble("fileC.s", srcC)
	if err != nil {
		t.Fatalf("failed to assemble fileC: %v", err)
	}
	objD, err := asm.Assemble("fileD.s", srcD)
	if err != nil {
		t.Fatalf("failed to assemble fileD: %v", err)
	}

	l := NewLinker(objA, objB, objC, objD)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link: %v", err)
	}

	// Bank 1 should contain auto_b ($BB x 100) then auto_c ($CC x 50)
	bank1Offset := INESHeaderSize + 1*BankSize
	if rom[bank1Offset] != 0xBB || rom[bank1Offset+99] != 0xBB {
		t.Errorf("expected auto_b in Bank 1 at offset 0, got 0x%02X", rom[bank1Offset])
	}
	if rom[bank1Offset+100] != 0xCC || rom[bank1Offset+149] != 0xCC {
		t.Errorf("expected auto_c in Bank 1 at offset 100, got 0x%02X", rom[bank1Offset+100])
	}

	// Bank 63 reset_handler: LDA #$01 (A9 01), LDX #$01 (A2 01), RTS (60)
	bank63Offset := INESHeaderSize + 63*BankSize
	expectedBank63 := []byte{0xA9, 0x01, 0xA2, 0x01, 0x60}
	for i, exp := range expectedBank63 {
		if rom[bank63Offset+i] != exp {
			t.Errorf("byte %d in bank 63 mismatch: got 0x%02X, want 0x%02X", i, rom[bank63Offset+i], exp)
		}
	}
}

func TestLinkAutoBankOverflow(t *testing.T) {
	src := `
    .export reset_handler
.bank auto
large_auto:
    .res 8193, $00
.bank 63
reset_handler:
    RTS
`
	objFile, err := asm.Assemble("overflow_auto.s", src)
	if err != nil {
		t.Fatalf("failed to assemble: %v", err)
	}

	l := NewLinker(objFile)
	_, err = l.Link()
	if err == nil {
		t.Fatal("expected auto bank overflow error, got nil")
	}
	if !strings.Contains(err.Error(), "exceeds bank limit") {
		t.Errorf("expected exceeds bank limit error message, got: %v", err)
	}
}

func TestLinkExportedDefine(t *testing.T) {
	srcA := `
    .export MAX_HP, PALETTE_BASE
    .define MAX_HP 100
    .define PALETTE_BASE $3F00
`
	srcB := `
    .export reset_handler
    .import MAX_HP, PALETTE_BASE

.bank 0
data_block:
    .byte MAX_HP
    .word PALETTE_BASE

.bank 63
reset_handler:
    LDA #MAX_HP
    LDX #<PALETTE_BASE
    LDY #>PALETTE_BASE
    RTS
`
	objA, err := asm.Assemble("constants.s", srcA)
	if err != nil {
		t.Fatalf("failed to assemble constants.s: %v", err)
	}
	objB, err := asm.Assemble("game.s", srcB)
	if err != nil {
		t.Fatalf("failed to assemble game.s: %v", err)
	}

	l := NewLinker(objA, objB)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link: %v", err)
	}

	// Verify bank 0 data: MAX_HP (100 = 0x64), PALETTE_BASE ($3F00 -> 00, 3F)
	bank0Offset := INESHeaderSize + 0*BankSize
	if rom[bank0Offset] != 100 {
		t.Errorf("expected byte 100, got %d", rom[bank0Offset])
	}
	if rom[bank0Offset+1] != 0x00 || rom[bank0Offset+2] != 0x3F {
		t.Errorf("expected word $3F00, got 0x%02X 0x%02X", rom[bank0Offset+1], rom[bank0Offset+2])
	}

	// Verify bank 63 reset_handler:
	// LDA #100       -> A9 64
	// LDX #<$3F00    -> A2 00
	// LDY #>$3F00    -> A0 3F
	// RTS            -> 60
	bank63Offset := INESHeaderSize + 63*BankSize
	expected := []byte{0xA9, 100, 0xA2, 0x00, 0xA0, 0x3F, 0x60}
	for i, exp := range expected {
		if rom[bank63Offset+i] != exp {
			t.Errorf("byte %d in bank 63 mismatch: got 0x%02X, want 0x%02X", i, rom[bank63Offset+i], exp)
		}
	}
}

func TestLinkMRT0DefaultStubbing(t *testing.T) {
	// Program defining only _main, without nmi or irq
	src := `
.bank 0
.proc _main
    LDA #$42
    RTS
.endproc
`
	objFile, err := asm.Assemble("game.s", src)
	if err != nil {
		t.Fatalf("failed to assemble: %v", err)
	}

	l := NewLinker(objFile)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link with mrt0: %v", err)
	}

	if len(rom) != TotalOutputSize {
		t.Fatalf("expected ROM size %d, got %d", TotalOutputSize, len(rom))
	}

	// Verify Bank 63 vectors
	bank63Offset := INESHeaderSize + 63*BankSize
	nmiAddr := binary.LittleEndian.Uint16(rom[bank63Offset+NMIVectorOffset : bank63Offset+NMIVectorOffset+2])
	resetAddr := binary.LittleEndian.Uint16(rom[bank63Offset+ResetVectorOffset : bank63Offset+ResetVectorOffset+2])
	irqAddr := binary.LittleEndian.Uint16(rom[bank63Offset+IRQVectorOffset : bank63Offset+IRQVectorOffset+2])

	// Vectors should point inside Bank 63 ($E000 - $FFFF)
	if resetAddr < 0xE000 {
		t.Errorf("reset vector $%04X is not in Bank 63", resetAddr)
	}
	if nmiAddr < 0xE000 {
		t.Errorf("nmi vector $%04X is not in Bank 63", nmiAddr)
	}
	if irqAddr < 0xE000 {
		t.Errorf("irq vector $%04X is not in Bank 63", irqAddr)
	}

	// Verify Bank 0 contains _main at $8000 (A9 42, 60)
	bank0Offset := INESHeaderSize
	if rom[bank0Offset] != 0xA9 || rom[bank0Offset+1] != 0x42 || rom[bank0Offset+2] != 0x60 {
		t.Errorf("expected _main at $8000 (A9 42 60), got %02X %02X %02X",
			rom[bank0Offset], rom[bank0Offset+1], rom[bank0Offset+2])
	}
}

func TestLinkMRT0CustomNMIAndIRQ(t *testing.T) {
	// Program defining _main, _nmi, and _irq
	src := `
.bank 0
.proc _main
    RTS
.endproc

.proc _nmi
    INC $00
    RTS
.endproc

.proc _irq
    INC $01
    RTS
.endproc
`
	objFile, err := asm.Assemble("handlers.s", src)
	if err != nil {
		t.Fatalf("failed to assemble: %v", err)
	}

	l := NewLinker(objFile)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link: %v", err)
	}

	if len(rom) != TotalOutputSize {
		t.Fatalf("expected ROM size %d, got %d", TotalOutputSize, len(rom))
	}
}

func TestLinkMRT0MissingMainError(t *testing.T) {
	// Object without _main or main
	src := `
.bank 0
.proc _some_func
    RTS
.endproc
`
	objFile, err := asm.Assemble("nomain.s", src)
	if err != nil {
		t.Fatalf("failed to assemble: %v", err)
	}

	l := NewLinker(objFile)
	_, err = l.Link()
	if err == nil {
		t.Fatal("expected linker error due to missing main, got nil")
	}
	if !strings.Contains(err.Error(), "undefined symbol \"main\"") {
		t.Errorf("expected undefined symbol main error, got: %v", err)
	}
}

func TestLinkEndToEndM3Program(t *testing.T) {
	m3Src := `
package main

define (
    PPU_CTRL $2000
)

var (
    frame_count uint16 zp
)

func nmi() {
    frame_count++
}

func main() bank 0 {
    for {
        asm {
        :   BIT $2002
            BPL :-
        }
    }
}
`
	_, asmCode, err := compiler.Compile("test.m3", m3Src)
	if err != nil {
		t.Fatalf("failed to compile m3 source: %v", err)
	}

	objFile, err := asm.Assemble("test.s", asmCode)
	if err != nil {
		t.Fatalf("failed to assemble generated asm:\n%s\nerror: %v", asmCode, err)
	}

	l := NewLinker(objFile)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link: %v", err)
	}

	if len(rom) != TotalOutputSize {
		t.Fatalf("expected ROM size %d, got %d", TotalOutputSize, len(rom))
	}
}

func TestLinkMRT0OAMFunctions(t *testing.T) {
	src := `
.import _oam_clear, _oam_advance_flicker, _oam_spr, _oam_spr_attr

.bank 0
.proc _main
    ; 1. Clear OAM
    JSR _oam_clear

    ; 2. Write a single sprite (X=120, Y=100, Tile=$01, Attr=$00)
    LDA #$00
    STA _oam_spr_attr
    LDA #120
    LDX #100
    LDY #$01
    JSR _oam_spr

    ; 3. Advance anti-flicker state
    JSR _oam_advance_flicker
    RTS
.endproc
`
	objFile, err := asm.Assemble("game.s", src)
	if err != nil {
		t.Fatalf("failed to assemble: %v", err)
	}

	l := NewLinker(objFile)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link with mrt0 OAM functions: %v", err)
	}

	if len(rom) != TotalOutputSize {
		t.Fatalf("expected ROM size %d, got %d", TotalOutputSize, len(rom))
	}

	// Verify that OAM functions were resolved and placed in Bank 63
	bank63Offset := INESHeaderSize + 63*BankSize
	resetAddr := binary.LittleEndian.Uint16(rom[bank63Offset+ResetVectorOffset : bank63Offset+ResetVectorOffset+2])
	if resetAddr < 0xE000 {
		t.Errorf("expected reset vector in Bank 63 ($E000-$FFFF), got $%04X", resetAddr)
	}
}

func TestCompileAndLinkOAMLibrary(t *testing.T) {
	oamContent, err := data.FS.ReadFile("lib/oam.m3")
	if err != nil {
		t.Fatalf("failed to read embedded lib/oam.m3: %v", err)
	}

	_, oamAsm, err := compiler.Compile("lib/oam.m3", string(oamContent))
	if err != nil {
		t.Fatalf("failed to compile lib/oam.m3: %v", err)
	}

	oamObj, err := asm.Assemble("oam.s", oamAsm)
	if err != nil {
		t.Fatalf("failed to assemble oam.s:\n%s\nerror: %v", oamAsm, err)
	}

	gameM3 := `
package main

func main() bank 0 {
    asm {
        JSR _Clear
        LDA #$00
        STA _oam_spr_attr
        LDA #100
        LDX #50
        LDY #$10
        JSR _PutSprite
        JSR _AdvanceFlicker
    }
}
`
	_, gameAsm, err := compiler.Compile("game.m3", gameM3)
	if err != nil {
		t.Fatalf("failed to compile game.m3: %v", err)
	}

	gameObj, err := asm.Assemble("game.s", gameAsm)
	if err != nil {
		t.Fatalf("failed to assemble game.s: %v", err)
	}

	l := NewLinker(gameObj, oamObj)
	rom, err := l.Link()
	if err != nil {
		t.Fatalf("failed to link game with oam library: %v", err)
	}

	if len(rom) != TotalOutputSize {
		t.Fatalf("expected ROM size %d, got %d", TotalOutputSize, len(rom))
	}
}



