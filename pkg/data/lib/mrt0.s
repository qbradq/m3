; ==============================================================================
; m3 Runtime (mrt0.s) - NES MMC3 (Mapper 4) Target
; ==============================================================================
; Platform and memory manager initialization for m3 programs.
;
; Provides:
;   - reset_handler: Hardware initialization, memory clear, MMC3 configuration,
;                    and jump to m3 main entry point.
;   - nmi_handler:   Context save, OAM DMA transfer, call _nmi (or stub),
;                    context restore, RTI.
;   - irq_handler:   Context save, call _irq (or stub), context restore, RTI.
;   - oam_clear:     Hides all 64 sprites and resets OAM write offset.
;   - oam_advance_flicker: Advances anti-flicker offset for sprite cycling.
;   - oam_spr:       Writes a single sprite to OAM at next available location.
; ==============================================================================

.export reset_handler, nmi_handler, irq_handler, __mrt0_stub_rts
.export oam_clear, _oam_clear
.export oam_advance_flicker, _oam_advance_flicker, oam_flicker, _oam_flicker
.export oam_spr, _oam_spr, oam_put_sprite, _oam_put_sprite
.export oam_off, _oam_off, oam_flicker_offset, _oam_flicker_offset, oam_spr_attr, _oam_spr_attr
.export reg_a_shadow, reg_x_shadow, reg_y_shadow, _reg_a_shadow, _reg_x_shadow, _reg_y_shadow, __arg0
.import _main, _main_main
.import _nmi
.import _irq

; ==============================================================================
; Zero Page Variables
; ==============================================================================
.zp
oam_off:            .res 1      ; Current OAM buffer write offset ($00-$FC)
oam_flicker_offset: .res 1      ; Anti-flicker frame starting offset ($00-$FC)
oam_spr_attr:       .res 1      ; Attribute argument for oam_spr
reg_a_shadow:       .res 1      ; Register A shadow / temporary
reg_x_shadow:       .res 1      ; Register X shadow / temporary
reg_y_shadow:       .res 1      ; Register Y shadow / temporary
__arg0:             .res 1      ; Fastcall 4th parameter scratchpad

; ==============================================================================
; Hardware Register Definitions
; ==============================================================================

; NES PPU Registers
PPU_CTRL         = $2000
PPU_MASK         = $2001
PPU_STATUS       = $2002
OAM_ADDR         = $2003
OAM_DATA         = $2004
PPU_SCROLL       = $2005
PPU_ADDR         = $2006
PPU_DATA         = $2007

; NES APU & I/O Registers
APU_STATUS       = $4015
APU_DMC_CTRL     = $4010
JOYPAD1          = $4016
JOYPAD2          = $4017
OAM_DMA          = $4014

; MMC3 Mapper 4 Registers
MMC3_BANK_SELECT = $8000
MMC3_BANK_DATA   = $8001
MMC3_MIRRORING   = $A000
MMC3_WRAM_PROT   = $A001
MMC3_IRQ_LATCH   = $C000
MMC3_IRQ_RELOAD  = $C001
MMC3_IRQ_DISABLE = $E000
MMC3_IRQ_ENABLE  = $E001

; ==============================================================================
; Fixed Bank 63 ($E000 - $FFFF)
; ==============================================================================
.bank 63

; ------------------------------------------------------------------------------
; Reset Handler: Platform and Memory Manager Initialization
; ------------------------------------------------------------------------------
reset_handler:
    SEI                         ; Disable IRQs
    CLD                         ; Clear decimal mode
    LDX #$FF
    TXS                         ; Initialize stack pointer to $FF

    INX                         ; X = 0
    STX PPU_CTRL                ; Disable NMI
    STX PPU_MASK                ; Disable rendering
    STX APU_STATUS              ; Disable APU sound channels
    STX APU_DMC_CTRL            ; Disable DMC IRQs

    ; 1st VBLANK wait (wait for PPU warm-up)
:   BIT PPU_STATUS
    BPL :-

    ; Clear CPU RAM ($0000 - $07FF)
    TXA
@clear_ram:
    STA $0000, X
    STA $0100, X
    STA $0200, X
    STA $0300, X
    STA $0400, X
    STA $0500, X
    STA $0600, X
    STA $0700, X
    INX
    BNE @clear_ram

    ; 2nd VBLANK wait (PPU is now fully ready)
:   BIT PPU_STATUS
    BPL :-

    ; Configure MMC3 Memory Map (Mode 0 PRG & CHR)
    ; PRG R6 ($8000-$9FFF) -> Bank 0
    LDA #$06
    STA MMC3_BANK_SELECT
    LDA #$00
    STA MMC3_BANK_DATA

    ; PRG R7 ($A000-$BFFF) -> Main function code bank
    LDA #$07
    STA MMC3_BANK_SELECT
    LDA #^_main_main
    STA MMC3_BANK_DATA

    ; CHR Banking (1:1 8KB CHR-RAM mapping)
    LDA #$00                    ; Select CHR R0
    STA MMC3_BANK_SELECT
    LDA #$00                    ; 2KB CHR Bank 0 ($0000)
    STA MMC3_BANK_DATA

    LDA #$01                    ; Select CHR R1
    STA MMC3_BANK_SELECT
    LDA #$02                    ; 2KB CHR Bank 2 ($0800)
    STA MMC3_BANK_DATA

    LDA #$02                    ; Select CHR R2
    STA MMC3_BANK_SELECT
    LDA #$04                    ; 1KB CHR Bank 4 ($1000)
    STA MMC3_BANK_DATA

    LDA #$03                    ; Select CHR R3
    STA MMC3_BANK_SELECT
    LDA #$05                    ; 1KB CHR Bank 5 ($1400)
    STA MMC3_BANK_DATA

    LDA #$04                    ; Select CHR R4
    STA MMC3_BANK_SELECT
    LDA #$06                    ; 1KB CHR Bank 6 ($1800)
    STA MMC3_BANK_DATA

    LDA #$05                    ; Select CHR R5
    STA MMC3_BANK_SELECT
    LDA #$07                    ; 1KB CHR Bank 7 ($1C00)
    STA MMC3_BANK_DATA

    ; Configure Mirroring: Vertical Mirroring (0 = Vertical)
    LDA #$00
    STA MMC3_MIRRORING

    ; Enable 8KB WRAM at $6000-$7FFF with write-protect disabled
    LDA #$80
    STA MMC3_WRAM_PROT

    ; Disable and acknowledge MMC3 scanline IRQs
    LDA #$00
    STA MMC3_IRQ_DISABLE

    ; Initialize OAM buffer so sprites start hidden offscreen
    JSR oam_clear

    ; Switch PRG R7 ($A000-$BFFF) to bank containing main function
    LDA #$07
    STA MMC3_BANK_SELECT
    LDA #^_main_main
    STA MMC3_BANK_DATA

    ; Call m3 main function
    JSR _main_main

    ; Infinite loop if main returns
:   JMP :-

; ------------------------------------------------------------------------------
; NMI Interrupt Handler
; ------------------------------------------------------------------------------
nmi_handler:
    PHA                         ; Save Accumulator
    TXA
    PHA                         ; Save X Register
    TYA
    PHA                         ; Save Y Register

    ; Trigger OAM DMA Transfer ($0200 -> PPU OAM)
    LDA #$00
    STA OAM_ADDR
    LDA #$02
    STA OAM_DMA

    JSR _nmi                    ; Call m3 nmi handler (or stub)

    PLA
    TAY                         ; Restore Y Register
    PLA
    TAX                         ; Restore X Register
    PLA                         ; Restore Accumulator
    RTI

; ------------------------------------------------------------------------------
; IRQ Interrupt Handler
; ------------------------------------------------------------------------------
irq_handler:
    PHA                         ; Save Accumulator
    TXA
    PHA                         ; Save X Register
    TYA
    PHA                         ; Save Y Register

    JSR _irq                    ; Call m3 irq handler (or stub)

    PLA
    TAY                         ; Restore Y Register
    PLA
    TAX                         ; Restore X Register
    PLA                         ; Restore Accumulator
    RTI

; ------------------------------------------------------------------------------
; OAM Management & Sprite Anti-Flicker Routines
; ------------------------------------------------------------------------------

; ------------------------------------------------------------------------------
; oam_clear (_oam_clear):
; Hides all 64 sprites in the OAM buffer ($0200-$02FF) by setting their Y
; coordinate to $FF (offscreen) and resets the current write offset (oam_off)
; to the current anti-flicker starting offset (oam_flicker_offset).
; ------------------------------------------------------------------------------
_oam_clear:
oam_clear:
    LDX #$00
    LDA #$FF
@clear_loop:
    STA $0200, X                ; Set sprite Y to $FF (offscreen)
    INX
    INX
    INX
    INX
    BNE @clear_loop

    LDA oam_flicker_offset
    STA oam_off                 ; Reset write offset to anti-flicker base
    RTS

; ------------------------------------------------------------------------------
; oam_advance_flicker (_oam_advance_flicker, oam_flicker, _oam_flicker):
; Advances the anti-flicker starting offset for the next frame.
; Steps by 17 sprites (68 bytes = $44), which is coprime to 64, ensuring
; an even rotation of hardware sprite priorities across frames.
; ------------------------------------------------------------------------------
_oam_advance_flicker:
oam_advance_flicker:
_oam_flicker:
oam_flicker:
    LDA oam_flicker_offset
    CLC
    ADC #$44                    ; Advance by 17 sprites ($44 bytes)
    STA oam_flicker_offset
    RTS

; ------------------------------------------------------------------------------
; oam_spr (_oam_spr, oam_put_sprite, _oam_put_sprite):
; Writes a single 8x8 sprite into the OAM buffer ($0200-$02FF) at the next
; available location (oam_off) and advances oam_off by 4.
;
; Calling Convention (Fastcall / m3 ABI):
;   A:            Sprite X coordinate (0-255)
;   X:            Sprite Y coordinate (0-255)
;   Y:            Sprite Tile index   (0-255)
;   oam_spr_attr: Sprite Attributes   (palette, flip, priority)
;
; OAM Buffer Layout:
;   $0200 + offset + 0: Y coordinate
;   $0200 + offset + 1: Tile index
;   $0200 + offset + 2: Attributes
;   $0200 + offset + 3: X coordinate
; ------------------------------------------------------------------------------
_oam_put_sprite:
oam_put_sprite:
_oam_spr:
oam_spr:
    PHA                         ; Save X coordinate (from A)
    TYA
    PHA                         ; Save Tile index (from Y)
    TXA
    PHA                         ; Save Y coordinate (from X)

    LDX oam_off                 ; Load current OAM buffer offset

    PLA
    STA $0200, X                ; Byte 0: Y coordinate
    INX

    PLA
    STA $0200, X                ; Byte 1: Tile index
    INX

    LDA oam_spr_attr
    STA $0200, X                ; Byte 2: Attributes
    INX

    PLA
    STA $0200, X                ; Byte 3: X coordinate
    INX

    STX oam_off                 ; Advance write offset (wraps at 256)
    RTS

; ------------------------------------------------------------------------------
; Default RTS Stub (used if _nmi or _irq are not defined)
; ------------------------------------------------------------------------------
__mrt0_stub_rts:
    RTS
