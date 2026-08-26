; ==============================================================================
; m3 Runtime (mrt0.s) - NES MMC3 (Mapper 4) Target
; ==============================================================================
; Platform and memory manager initialization for m3 programs.
;
; Provides:
;   - reset_handler: Hardware initialization, memory clear, MMC3 configuration,
;                    and jump to m3 main entry point.
;   - nmi_handler:   Context save, call _nmi (or stub), context restore, RTI.
;   - irq_handler:   Context save, call _irq (or stub), context restore, RTI.
; ==============================================================================

.export reset_handler, nmi_handler, irq_handler, __mrt0_stub_rts
.import _main
.import _nmi
.import _irq

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

    ; PRG R7 ($A000-$BFFF) -> Bank 1
    LDA #$07
    STA MMC3_BANK_SELECT
    LDA #$01
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

    ; Call m3 main function
    JSR _main

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
; Default RTS Stub (used if _nmi or _irq are not defined)
; ------------------------------------------------------------------------------
__mrt0_stub_rts:
    RTS
