package ppu

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

// Zero-page scratchpad variables for PPU streaming operations
var (
    pal_ptr    *uint8 zp
    upload_src *uint8 zp
    upload_dst uint16 zp
    upload_len uint16 zp
)

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
        LDA #%10000000
        STA $2000
        LDA #$00
        STA $2005
        STA $2005
    }
}

// DirectUploadPalette uploads the entire palette pointed to by pal to the PPU
// immediately. Should only be called when the PPU is disabled.
//
// Fastcall Parameters (m3 ABI):
//   pal: Low byte in A, High byte in X (or _ppu_pal_ptr in ZP)
func DirectUploadPalette(pal *uint8[32]) {
    asm {
        STA _ppu_pal_ptr
        STX _ppu_pal_ptr+1

        BIT $2002
        LDA #$3F
        STA $2006
        LDA #$00
        STA $2006

        LDY #$00
    :   LDA (_ppu_pal_ptr), Y
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
// Fastcall / Memory Parameters:
//   src:     Source address pointer (_ppu_upload_src in ZP)
//   ppu_dst: Target PPU VRAM address (_ppu_upload_dst in ZP)
//   len:     Number of bytes to upload (_ppu_upload_len in ZP)
func DirectUpload(src *uint8[], ppu_dst, len uint16) {
    asm {
        BIT $2002
        LDA _ppu_upload_dst+1
        STA $2006
        LDA _ppu_upload_dst
        STA $2006

        ; Check if high byte of length > 0
        LDX _ppu_upload_len+1
        BEQ @copy_remainder

        ; Stream 256-byte full pages
    @page_loop:
        LDY #$00
    @page_byte:
        LDA (_ppu_upload_src), Y
        STA $2007
        INY
        BNE @page_byte

        INC _ppu_upload_src+1
        DEX
        BNE @page_loop

    @copy_remainder:
        LDX _ppu_upload_len
        BEQ @done
        LDY #$00
    @rem_byte:
        LDA (_ppu_upload_src), Y
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
