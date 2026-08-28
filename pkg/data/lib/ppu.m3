package ppu

bank 63

// =============================================================================
// m3 Standard Library - PPU Control & Palette Manager (lib/ppu.m3)
// =============================================================================

// PPU Hardware Register Base Addresses
define (
    PPU_CTRL   $2000
    PPU_MASK   $2001
    PPU_STATUS $2002
    OAM_ADDR   $2003
    OAM_DATA   $2004
    PPU_SCROLL $2005
    PPU_ADDR   $2006
    PPU_DATA   $2007
)

// WaitForVBlank polls the PPU status register until the vertical blanking
// interval starts.
func WaitForVBlank() {
    asm {
    :   BIT $2002
        BPL :-
    }
}

// Disable turns the screen and NMI processing off at the end of the next
// NMI. Waits until the end of the next NMI before returning.
func Disable() {
    asm {
    :   BIT $2002
        BPL :-

        LDA #$00
        STA $2001
        STA $2000
    }
}

// Enable turns the screen and NMI processing back on.
func Enable() {
    asm {
        BIT $2002
        LDA #%00011110
        STA $2001
        LDA #%10001000
        STA $2000
        LDA #$00
        STA $2005
        STA $2005
    }
}

// DirectUploadPalette uploads the entire palette pointed to by pal to the PPU
// immediately. Should only be called when the PPU is disabled.
//
// Fastcall Parameters (3-byte register):
//   pal: Registers A (low), X (high)
func DirectUploadPalette(pal *uint8[32]) {
    asm {
        STA __leaf_param0
        STX __leaf_param1

        BIT $2002
        LDA #$3F
        STA $2006
        LDA #$00
        STA $2006

        LDY #$00
    :   LDA (__leaf_param0), Y
        STA $2007
        INY
        CPY #32
        BNE :-

        LDA #$00
        STA $2005
        STA $2005
    }
}

// DirectUpload streams len bytes from src directly into PPU VRAM starting at
// ppu_dst. Should only be called when the PPU is disabled.
//
// Fastcall Parameters (3-byte register + excess ZP):
//   src:     Registers A (low), X (high)
//   ppu_dst: __leaf_param0 (low), __leaf_param1 (high)
//   len:     __leaf_param2 (low), __leaf_param3 (high)
func DirectUpload(src *uint8[], ppu_dst, len uint16) {
    asm {
        STA __leaf_param4
        STX __leaf_param5

        BIT $2002
        LDA __leaf_param1
        STA $2006
        LDA __leaf_param0
        STA $2006

        ; Check if high byte of length > 0
        LDX __leaf_param3
        BEQ @copy_remainder

        ; Stream 256-byte full pages
    @page_loop:
        LDY #$00
    @page_byte:
        LDA (__leaf_param4), Y
        STA $2007
        INY
        BNE @page_byte

        INC __leaf_param5
        DEX
        BNE @page_loop

    @copy_remainder:
        LDX __leaf_param2
        BEQ @done
        LDY #$00
    @rem_byte:
        LDA (__leaf_param4), Y
        STA $2007
        INY
        DEX
        BNE @rem_byte

    @done:
        LDA #$00
        STA $2005
        STA $2005
    }
}
