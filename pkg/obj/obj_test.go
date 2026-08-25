package obj

import (
	"bytes"
	"reflect"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	orig := NewObjectFile("player.m3")
	orig.AddSymbol(Symbol{
		Name:  "player_init",
		Type:  SymbolTypeLabel,
		Scope: ScopeExport,
		Bank:  1,
		Value: 0x8000,
	})
	orig.AddSymbol(Symbol{
		Name:  "ppu_sync",
		Type:  SymbolTypeImport,
		Scope: ScopeGlobal,
		Bank:  -1,
		Value: 0,
	})

	bank0 := orig.GetOrCreateBank(0)
	bank0.Data = []byte{0xA9, 0x00, 0x8D, 0x00, 0x20}
	bank0.Relocations = append(bank0.Relocations, Relocation{
		Offset: 3,
		Symbol: "PPU_CTRL",
		Type:   RelocAddr16,
		Addend: 0,
	})

	var buf bytes.Buffer
	if err := orig.Encode(&buf); err != nil {
		t.Fatalf("encode error: %v", err)
	}

	decoded, err := Decode(&buf)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if decoded.SourceFile != orig.SourceFile {
		t.Errorf("SourceFile mismatch: got %s, want %s", decoded.SourceFile, orig.SourceFile)
	}

	if !reflect.DeepEqual(decoded.Symbols, orig.Symbols) {
		t.Errorf("Symbols mismatch:\ngot  %+v\nwant %+v", decoded.Symbols, orig.Symbols)
	}

	if len(decoded.Banks) != len(orig.Banks) {
		t.Fatalf("Banks count mismatch: got %d, want %d", len(decoded.Banks), len(orig.Banks))
	}

	for i := range orig.Banks {
		if decoded.Banks[i].BankIndex != orig.Banks[i].BankIndex {
			t.Errorf("Bank[%d] index mismatch", i)
		}
		if !bytes.Equal(decoded.Banks[i].Data, orig.Banks[i].Data) {
			t.Errorf("Bank[%d] data mismatch", i)
		}
		if !reflect.DeepEqual(decoded.Banks[i].Relocations, orig.Banks[i].Relocations) {
			t.Errorf("Bank[%d] relocations mismatch", i)
		}
	}
}
