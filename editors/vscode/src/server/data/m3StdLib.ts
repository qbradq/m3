export const M3_STDLIB: Record<string, string> = {
  'ppu.m3': `package ppu

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
        LDA #%10000000
        STA $2000
        LDA #$00
        STA $2005
        STA $2005
    }
}

// UploadPalette uploads the entire palette pointed to by pal to the PPU
// immediately. Should only be called when the PPU is disabled.
func UploadPalette(pal uint8[32]) {
}

// DirectUploadPalette uploads the entire palette pointed to by pal to the PPU
// immediately. Should only be called when the PPU is disabled.
//
// Fastcall Parameters (m3 ABI):
//   pal: Low byte in A, High byte in X (or _pal_ptr in ZP)
func DirectUploadPalette(pal *uint8[32]) {
    asm {
        STA _pal_ptr
        STX _pal_ptr+1

        BIT $2002
        LDA #$3F
        STA $2006
        LDA #$00
        STA $2006

        LDY #$00
    :   LDA (_pal_ptr), Y
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
//   src:     Source address pointer (_upload_src in ZP)
//   ppu_dst: Target PPU VRAM address (_upload_dst in ZP)
//   len:     Number of bytes to upload (_upload_len in ZP)
func DirectUpload(src *uint8[], ppu_dst, len uint16) {
    asm {
        BIT $2002
        LDA _upload_dst+1
        STA $2006
        LDA _upload_dst
        STA $2006

        LDX _upload_len+1
        BEQ @copy_remainder

    @page_loop:
        LDY #$00
    @page_byte:
        LDA (_upload_src), Y
        STA $2007
        INY
        BNE @page_byte

        INC _upload_src+1
        DEX
        BNE @page_loop

    @copy_remainder:
        LDX _upload_len
        BEQ @done
        LDY #$00
    @rem_byte:
        LDA (_upload_src), Y
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
`,

  'oam.m3': `package oam

// OAM Sprite Buffer Base Address
define OAM_BUFFER $0200

// Sprite Attribute Flag Constants
define (
    SPR_PAL0     0          // Palette 0
    SPR_PAL1     1          // Palette 1
    SPR_PAL2     2          // Palette 2
    SPR_PAL3     3          // Palette 3
    SPR_BEHIND   %00100000  // Priority: behind background
    SPR_FLIP_H   %01000000  // Flip sprite horizontally
    SPR_FLIP_V   %10000000  // Flip sprite vertically
)

// Clear hides all 64 sprites in the OAM buffer (setting Y = $FF) and resets
// the write pointer to the current anti-flicker base offset.
func Clear() {
    asm {
        JSR _oam_clear
    }
}

// AdvanceFlicker steps the anti-flicker starting offset by 17 sprites ($44 bytes),
// rotating hardware sprite priority to prevent scanline dropout across frames.
func AdvanceFlicker() {
    asm {
        JSR _oam_advance_flicker
    }
}

// PutSprite writes a single 8x8 sprite into the OAM buffer at the next available
// position and advances the write pointer by 4 bytes.
//
// Fastcall Parameters (m3 ABI):
//   x:    Accumulator A
//   y:    Register X
//   tile: Register Y
//   attr: Memory __arg0 / _oam_spr_attr
func PutSprite(x uint8, y uint8, tile uint8, attr uint8) {
    asm {
        JSR _oam_spr
    }
}
`,

  'memory.m3': `package memory

// Zero Page Scratchpad Variables for Memory Transfers
var (
    src_ptr *uint8  zp
    dst_ptr *uint8  zp
    len_cnt uint16 zp
)

// Copy copies len bytes from src to dst. The behavior is undefined if src
// overlaps dst.
//
// Fastcall / Memory Parameters:
//   src: Source address pointer (_src_ptr in ZP)
//   dst: Destination address pointer (_dst_ptr in ZP)
//   len: Number of bytes to copy (_len_cnt in ZP)
func Copy(src, dst *uint8[], len uint16) {
    asm {
        LDX _len_cnt+1
        BEQ @copy_remainder

    @page_loop:
        LDY #$00
    @page_byte:
        LDA (_src_ptr), Y
        STA (_dst_ptr), Y
        INY
        BNE @page_byte

        INC _src_ptr+1
        INC _dst_ptr+1
        DEX
        BNE @page_loop

    @copy_remainder:
        LDX _len_cnt
        BEQ @done
        LDY #$00
    @rem_byte:
        LDA (_src_ptr), Y
        STA (_dst_ptr), Y
        INY
        DEX
        BNE @rem_byte

    @done:
    }
}
`,

  'ppu_driver.m3': `package ppu_driver

// Hardware Registers & Command Constants
define (
    PPU_CTRL   $2000
    PPU_MASK   $2001
    PPU_STATUS $2002
    PPU_SCROLL $2005
    PPU_ADDR   $2006
    PPU_DATA   $2007

    CMD_END   $00
    CMD_HORIZ $01
    CMD_VERT  $02
    CMD_BYTE  $03
)

// Display list RAM buffer and zero-page scratchpad variables
var (
    list_buf uint8[128] ram
    list_len uint8      zp
    push_src *uint8     zp
    push_dst uint16     zp
    push_len uint8      zp
    push_val uint8      zp
)

// Clear resets the display list write pointer to 0 and clears the buffer head.
func Clear() {
}

// PushHorizontal buffers a command to copy len bytes from src to PPU RAM
// starting at dest with horizontal (+1) auto-increment during the next VBlank.
func PushHorizontal(src *uint8[], dest uint16, len uint8) {
}

// PushVertical buffers a command to copy len bytes from src to PPU RAM
// starting at dest with vertical (+32) auto-increment during the next VBlank.
func PushVertical(src *uint8[], dest uint16, len uint8) {
}

// PushByte buffers a single byte patch to PPU RAM at dest during the next VBlank.
func PushByte(val uint8, dest uint16) {
}

// PushPalette buffers a 32-byte palette update from src to PPU palette
// RAM ($3F00) during the next VBlank.
func PushPalette(src *uint8[32]) {
}

// Process iterates over all buffered display list commands, executes the PPU
// transfers, and clears the display list. Must be called during VBlank from the
// NMI handler with PPU enabled.
func Process() {
}
`,
};
