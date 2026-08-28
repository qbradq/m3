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
func DirectUploadPalette(pal *uint8[32]) {
}

// DirectUpload streams len bytes from src directly into PPU VRAM starting at
// ppu_dst. Should only be called when the PPU is disabled.
func DirectUpload(src *uint8[], ppu_dst, len uint16) {
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
func PutSprite(x uint8, y uint8, tile uint8, attr uint8) {
    asm {
        JSR _oam_spr
    }
}
`,

  'memory.m3': `package memory

// Copy copies len bytes from src to dst. The behavior is undefined if src
// overlaps dst.
func Copy(src, dst *uint8[], len uint16) {
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
