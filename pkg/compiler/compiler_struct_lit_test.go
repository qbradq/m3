package compiler

import (
	"strings"
	"testing"
)

func TestParseAndCompileConstStructLiteral(t *testing.T) {
	src := `package main

type Tile struct {
	chr uint8[4]
	palette uint8
	walkable bool
	sailable bool
}

const DeepWater Tile = {
	chr: [4]uint8{128, 128, 128, 128},
	palette: 1,
	walkable: false,
	sailable: true,
}

func main() {
}
`
	_, asmCode, err := Compile("main.m3", src)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Verify .code section and byte emission
	if !strings.Contains(asmCode, ".code") {
		t.Errorf("expected .code segment, got:\n%s", asmCode)
	}
	if !strings.Contains(asmCode, "_main_DeepWater:") {
		t.Errorf("expected symbol _main_DeepWater:, got:\n%s", asmCode)
	}
	// DeepWater fields: chr (4 bytes: $80,$80,$80,$80), palette ($01), walkable ($00), sailable ($01)
	if !strings.Contains(asmCode, ".byte $80, $80, $80, $80") {
		t.Errorf("expected .byte $80, $80, $80, $80, got:\n%s", asmCode)
	}
	if !strings.Contains(asmCode, ".byte $01") {
		t.Errorf("expected .byte $01, got:\n%s", asmCode)
	}
	if !strings.Contains(asmCode, ".byte $00") {
		t.Errorf("expected .byte $00, got:\n%s", asmCode)
	}
}

func TestParseAndCompileDataStructLiteral(t *testing.T) {
	src := `package tileset

type Tile struct {
	chr uint8[4]
	palette uint8
	walkable bool
	sailable bool
}

data SurfaceTile Tile bank 1 = {
	chr: {128, 129, 130, 131},
	palette: 2,
	walkable: true,
	sailable: false,
}
`
	_, asmCode, err := Compile("tileset.m3", src)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Verify bank, .data segment, and byte emission
	if !strings.Contains(asmCode, ".bank 1") {
		t.Errorf("expected .bank 1, got:\n%s", asmCode)
	}
	if !strings.Contains(asmCode, ".data") {
		t.Errorf("expected .data segment, got:\n%s", asmCode)
	}
	if !strings.Contains(asmCode, "_tileset_SurfaceTile:") {
		t.Errorf("expected symbol _tileset_SurfaceTile:, got:\n%s", asmCode)
	}
	if !strings.Contains(asmCode, ".byte $80, $81, $82, $83") {
		t.Errorf("expected .byte $80, $81, $82, $83, got:\n%s", asmCode)
	}
	if !strings.Contains(asmCode, ".byte $02") {
		t.Errorf("expected .byte $02, got:\n%s", asmCode)
	}
}

func TestStructLiteralOmittedFieldsAndReordered(t *testing.T) {
	src := `package main

type Tile struct {
	chr uint8[4]
	palette uint8
	walkable bool
	sailable bool
}

const Grass Tile = {
	walkable: true,
	palette: 2,
}
`
	_, asmCode, err := Compile("main.m3", src)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	// Omitted chr (4 zero bytes), palette ($02), walkable ($01), omitted sailable ($00)
	if !strings.Contains(asmCode, ".byte $00, $00, $00, $00") {
		t.Errorf("expected 4 zero bytes for omitted chr, got:\n%s", asmCode)
	}
	if !strings.Contains(asmCode, ".byte $02") {
		t.Errorf("expected .byte $02, got:\n%s", asmCode)
	}
	if !strings.Contains(asmCode, ".byte $01") {
		t.Errorf("expected .byte $01, got:\n%s", asmCode)
	}
}

func TestArrayOfStructLiterals(t *testing.T) {
	src := `package main

type Tile struct {
	chr uint8[4]
	palette uint8
	walkable bool
	sailable bool
}

data SurfaceTiles Tile[2] bank auto = [2]Tile{
	{
		chr: [4]uint8{128, 128, 128, 128},
		palette: 1,
		walkable: false,
		sailable: true,
	},
	{
		chr: [4]uint8{129, 129, 129, 129},
		palette: 1,
		walkable: false,
		sailable: true,
	},
}
`
	_, asmCode, err := Compile("main.m3", src)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	if !strings.Contains(asmCode, ".byte $80, $80, $80, $80") {
		t.Errorf("expected first element chr, got:\n%s", asmCode)
	}
	if !strings.Contains(asmCode, ".byte $81, $81, $81, $81") {
		t.Errorf("expected second element chr, got:\n%s", asmCode)
	}
}

func TestStructLiteralUnknownField(t *testing.T) {
	src := `package main

type Tile struct {
	palette uint8
}

const Invalid Tile = {
	nonExistent: 1,
}
`
	_, _, err := Compile("main.m3", src)
	if err == nil {
		t.Fatalf("expected error for unknown field in struct literal, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("expected unknown field error, got: %v", err)
	}
}

func TestStructLiteralDuplicateField(t *testing.T) {
	src := `package main

type Tile struct {
	palette uint8
}

const Invalid Tile = {
	palette: 1,
	palette: 2,
}
`
	_, _, err := Compile("main.m3", src)
	if err == nil {
		t.Fatalf("expected error for duplicate field in struct literal, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate field") {
		t.Errorf("expected duplicate field error, got: %v", err)
	}
}
