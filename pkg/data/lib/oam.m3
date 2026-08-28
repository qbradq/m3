package oam

bank 63

// =============================================================================
// m3 Standard Library - OAM Sprite & Anti-Flicker Manager (lib/oam.m3)
// =============================================================================

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
// Fastcall Parameters (3-byte register + excess ZP):
//   x:    Accumulator A
//   y:    Register X
//   tile: Register Y
//   attr: __leaf_param0
func PutSprite(x uint8, y uint8, tile uint8, attr uint8) {
    asm {
        JSR _oam_spr
    }
}
