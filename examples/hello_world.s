; ==============================================================================
; m3 NES MMC3 Hello World & Sprite Movement Demo
; ==============================================================================
; Target Specifications:
;   - NES (NTSC) with MMC3 Mapper (Mapper 4)
;   - 512KB PRG-ROM (64 x 8KB banks)
;   - 8KB CHR-RAM
;   - 8KB PRG-RAM (WRAM at $6000-$7FFF)
; ==============================================================================

.export main, reset_handler, nmi_handler, irq_handler

; ==============================================================================
; 1. Hardware Register Definitions
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

; Controller Button Masks (Shifted order: A, B, Select, Start, Up, Down, Left, Right)
BUTTON_A         = %10000000
BUTTON_B         = %01000000
BUTTON_SELECT    = %00100000
BUTTON_START     = %00010000
BUTTON_UP        = %00001000
BUTTON_DOWN      = %00000100
BUTTON_LEFT      = %00000010
BUTTON_RIGHT     = %00000001

; ==============================================================================
; 2. Zero-Page RAM Allocations ($0000 - $00FF)
; ==============================================================================
.zp
ptr_src_lo:    .res 1
ptr_src_hi:    .res 1
ptr_dst_lo:    .res 1
ptr_dst_hi:    .res 1
byte_count_lo: .res 1
byte_count_hi: .res 1

.res 10 ; Skip to $10 for game state

nmi_ready:     .res 1
buttons:       .res 1
player_x:      .res 1
player_y:      .res 1

; OAM buffer located in CPU RAM ($0200 - $02FF)
OAM_BUFFER       = $0200

; ==============================================================================
; 3. PRG Bank 0 ($8000 - $9FFF) - Main Code & Logic
; ==============================================================================
.bank 0

main:
    ; 1. Configure MMC3 Memory Map
    JSR setup_mmc3

    ; 2. Load Palettes (Background font palette + Sprite palette)
    JSR load_palettes

    ; 3. Upload CHR Data to CHR-RAM ($0000: Sprites, $1000: Font)
    JSR load_chr_ram

    ; 4. Clear Nametable 0 and Print "Hello, World!"
    JSR setup_nametable

    ; 5. Initialize Player Sprite Position & OAM Buffer
    JSR init_player_sprites

    ; 6. Enable PPU Graphics & NMI
    JSR enable_rendering

    ; 7. Enter Main Game Loop
game_loop:
    ; Wait for NMI frame sync
:   LDA nmi_ready
    BEQ :-
    LDA #$00
    STA nmi_ready

    ; Read Controller 1 Input
    JSR read_controller

    ; Update Player Movement
    JSR update_player

    ; Refresh OAM Sprite Buffer
    JSR update_player_sprites

    JMP game_loop

; ------------------------------------------------------------------------------
; MMC3 Setup Routine
; ------------------------------------------------------------------------------
setup_mmc3:
    ; Configure PRG Bank Switching (Mode 0):
    ; $8000-$9FFF -> Bank 0 (R6)
    ; $A000-$BFFF -> Bank 1 (R7, Asset Bank)
    ; $C000-$DFFF -> Fixed Bank 62
    ; $E000-$FFFF -> Fixed Bank 63
    LDA #$06            ; Select PRG R6 ($8000-$9FFF)
    STA MMC3_BANK_SELECT
    LDA #$00            ; Map PRG Bank 0
    STA MMC3_BANK_DATA

    LDA #$07            ; Select PRG R7 ($A000-$BFFF)
    STA MMC3_BANK_SELECT
    LDA #$01            ; Map PRG Bank 1
    STA MMC3_BANK_DATA

    ; Configure CHR-RAM Banking (Mode 0, 8KB CHR-RAM mapped 1:1):
    ; $0000-$07FF -> 2KB CHR Bank 0 (R0)
    ; $0800-$0FFF -> 2KB CHR Bank 2 (R1)
    ; $1000-$13FF -> 1KB CHR Bank 4 (R2)
    ; $1400-$17FF -> 1KB CHR Bank 5 (R3)
    ; $1800-$1BFF -> 1KB CHR Bank 6 (R4)
    ; $1C00-$1FFF -> 1KB CHR Bank 7 (R5)
    LDA #$00            ; Select CHR R0
    STA MMC3_BANK_SELECT
    LDA #$00            ; 2KB CHR Bank 0 ($0000)
    STA MMC3_BANK_DATA

    LDA #$01            ; Select CHR R1
    STA MMC3_BANK_SELECT
    LDA #$02            ; 2KB CHR Bank 2 ($0800)
    STA MMC3_BANK_DATA

    LDA #$02            ; Select CHR R2
    STA MMC3_BANK_SELECT
    LDA #$04            ; 1KB CHR Bank 4 ($1000)
    STA MMC3_BANK_DATA

    LDA #$03            ; Select CHR R3
    STA MMC3_BANK_SELECT
    LDA #$05            ; 1KB CHR Bank 5 ($1400)
    STA MMC3_BANK_DATA

    LDA #$04            ; Select CHR R4
    STA MMC3_BANK_SELECT
    LDA #$06            ; 1KB CHR Bank 6 ($1800)
    STA MMC3_BANK_DATA

    LDA #$05            ; Select CHR R5
    STA MMC3_BANK_SELECT
    LDA #$07            ; 1KB CHR Bank 7 ($1C00)
    STA MMC3_BANK_DATA

    ; Configure Mirroring: Vertical Mirroring (0 = Vertical, 1 = Horizontal)
    LDA #$00
    STA MMC3_MIRRORING

    ; Enable 8KB WRAM at $6000-$7FFF with write-protect disabled
    LDA #$80
    STA MMC3_WRAM_PROT

    ; Disable and acknowledge MMC3 scanline IRQs
    LDA #$00
    STA MMC3_IRQ_DISABLE

    RTS

; ------------------------------------------------------------------------------
; Palette Loading Routine
; ------------------------------------------------------------------------------
load_palettes:
    ; Set PPU address to Palette RAM ($3F00)
    LDA PPU_STATUS      ; Reset PPU latch
    LDA #$3F
    STA PPU_ADDR
    LDA #$00
    STA PPU_ADDR

    ; Upload Background Palette 0 (Font Palette - 4 bytes from Bank 1)
    LDX #$00
@bg_pal:
    LDA font_pal + $A000, X
    STA PPU_DATA
    INX
    CPX #$04
    BNE @bg_pal

    ; Fill unused background palettes with backdrop color
    LDY #12
    LDA font_pal + $A000
@fill_bg:
    STA PPU_DATA
    DEY
    BNE @fill_bg

    ; Upload Sprite Palette 0 (4 bytes from Bank 1)
    LDX #$00
@spr_pal:
    LDA sprite_pal + $A000, X
    STA PPU_DATA
    INX
    CPX #$04
    BNE @spr_pal

    ; Fill unused sprite palettes with backdrop color
    LDY #12
    LDA font_pal + $A000
@fill_spr:
    STA PPU_DATA
    DEY
    BNE @fill_spr

    RTS

; ------------------------------------------------------------------------------
; CHR-RAM Upload Routine
; ------------------------------------------------------------------------------
load_chr_ram:
    ; --- 1. Upload Sprite CHR to PPU $0000 ---
    LDA PPU_STATUS
    LDA #$00
    STA PPU_ADDR
    STA PPU_ADDR

    LDA #<(sprite_chr + $A000)
    STA ptr_src_lo
    LDA #>(sprite_chr + $A000)
    STA ptr_src_hi

    LDA #<(sprite_chr_end - sprite_chr)
    STA byte_count_lo
    LDA #>(sprite_chr_end - sprite_chr)
    STA byte_count_hi

    JSR copy_to_ppu

    ; --- 2. Upload Font CHR to PPU $1000 ---
    LDA PPU_STATUS
    LDA #$10
    STA PPU_ADDR
    LDA #$00
    STA PPU_ADDR

    LDA #<(font_chr + $A000)
    STA ptr_src_lo
    LDA #>(font_chr + $A000)
    STA ptr_src_hi

    LDA #<(font_chr_end - font_chr)
    STA byte_count_lo
    LDA #>(font_chr_end - font_chr)
    STA byte_count_hi

    JSR copy_to_ppu

    RTS

; ------------------------------------------------------------------------------
; Copy Buffer to PPU Data Port
; Inputs: ptr_src_lo/hi, byte_count_lo/hi
; ------------------------------------------------------------------------------
copy_to_ppu:
    LDY #$00
@loop:
    LDA byte_count_lo
    ORA byte_count_hi
    BEQ @done

    LDA (ptr_src_lo), Y
    STA PPU_DATA

    ; Increment 16-bit source pointer
    INC ptr_src_lo
    BNE :+
    INC ptr_src_hi

    ; Decrement 16-bit byte counter
:   LDA byte_count_lo
    BNE :+
    DEC byte_count_hi
:   DEC byte_count_lo
    JMP @loop

@done:
    RTS

; ------------------------------------------------------------------------------
; Nametable Setup & String Printing Routine
; ------------------------------------------------------------------------------
setup_nametable:
    ; Set PPU address to Nametable 0 ($2000)
    LDA PPU_STATUS
    LDA #$20
    STA PPU_ADDR
    LDA #$00
    STA PPU_ADDR

    ; Clear Nametable (960 tiles = 3 full 256-byte pages + 192 bytes)
    ; Fill with ASCII space ($20)
    LDA #$20            ; ASCII ' ' space character
    LDX #$03            ; 3 pages of 256
    LDY #$00
@clear_page:
    STA PPU_DATA
    INY
    BNE @clear_page
    DEX
    BNE @clear_page

    LDY #192            ; Remaining 192 tiles (960 total)
@clear_rem:
    STA PPU_DATA
    DEY
    BNE @clear_rem

    ; Clear Attribute Table (64 bytes of $00 at $23C0-$23FF)
    LDA #$00
    LDY #64
@clear_attr:
    STA PPU_DATA
    DEY
    BNE @clear_attr

    ; Print "Hello, World!" centered on screen:
    ; Screen is 32 columns wide, message is 13 characters.
    ; Row 14, Column 10 -> PPU address: $2000 + 14*32 + 10 = $21CA
    LDA PPU_STATUS
    LDA #$21
    STA PPU_ADDR
    LDA #$CA
    STA PPU_ADDR

    LDY #$00
@print_loop:
    LDA msg_hello, Y
    BEQ @print_done
    STA PPU_DATA
    INY
    JMP @print_loop

@print_done:
    RTS

; ------------------------------------------------------------------------------
; Player Sprite Initialization & Update
; ------------------------------------------------------------------------------
init_player_sprites:
    ; Start player sprite near center of the screen
    LDA #120
    STA player_x
    LDA #104
    STA player_y

    ; Hide all 64 sprites in OAM buffer ($0200-$02FF) by setting Y to $FF
    LDX #$00
    LDA #$FF
@hide_all:
    STA OAM_BUFFER, X
    INX
    INX
    INX
    INX
    BNE @hide_all

    ; Update the 4 hardware sprites for the 16x16 meta-sprite
    JSR update_player_sprites
    RTS

update_player_sprites:
    ; Sprite 0: Top-Left (Relative character #0)
    ; OAM layout: [Y, Tile, Attribute, X]
    LDA player_y
    STA OAM_BUFFER + 0          ; Y coordinate
    LDA #0                      ; Tile index 0
    STA OAM_BUFFER + 1
    LDA #0                      ; Attribute (palette 0, normal orientation)
    STA OAM_BUFFER + 2
    LDA player_x
    STA OAM_BUFFER + 3          ; X coordinate

    ; Sprite 1: Top-Right (Relative character #1)
    LDA player_y
    STA OAM_BUFFER + 4          ; Y coordinate
    LDA #1                      ; Tile index 1
    STA OAM_BUFFER + 5
    LDA #0                      ; Attribute
    STA OAM_BUFFER + 6
    LDA player_x
    CLC
    ADC #8
    STA OAM_BUFFER + 7          ; X + 8

    ; Sprite 2: Bottom-Left (Relative character #16)
    LDA player_y
    CLC
    ADC #8
    STA OAM_BUFFER + 8          ; Y + 8
    LDA #16                     ; Tile index 16
    STA OAM_BUFFER + 9
    LDA #0                      ; Attribute
    STA OAM_BUFFER + 10
    LDA player_x
    STA OAM_BUFFER + 11         ; X coordinate

    ; Sprite 3: Bottom-Right (Relative character #17)
    LDA player_y
    CLC
    ADC #8
    STA OAM_BUFFER + 12         ; Y + 8
    LDA #17                     ; Tile index 17
    STA OAM_BUFFER + 13
    LDA #0                      ; Attribute
    STA OAM_BUFFER + 14
    LDA player_x
    CLC
    ADC #8
    STA OAM_BUFFER + 15         ; X + 8

    RTS

; ------------------------------------------------------------------------------
; Controller Input Reader
; Standard strobe sequence on $4016
; ------------------------------------------------------------------------------
read_controller:
    LDA #$01
    STA JOYPAD1
    LDA #$00
    STA JOYPAD1

    LDX #$08
@read_bits:
    LDA JOYPAD1
    LSR A
    ROL buttons
    DEX
    BNE @read_bits
    RTS

; ------------------------------------------------------------------------------
; Player Movement Logic (D-Pad Control with Screen Bounds)
; ------------------------------------------------------------------------------
update_player:
    ; Up Button Check
    LDA buttons
    AND #BUTTON_UP
    BEQ @check_down
    LDA player_y
    CMP #8                      ; Top boundary
    BCC @check_down
    DEC player_y

@check_down:
    LDA buttons
    AND #BUTTON_DOWN
    BEQ @check_left
    LDA player_y
    CMP #216                    ; Bottom boundary (240 - 16 - 8)
    BCS @check_left
    INC player_y

@check_left:
    LDA buttons
    AND #BUTTON_LEFT
    BEQ @check_right
    LDA player_x
    CMP #8                      ; Left boundary
    BCC @check_right
    DEC player_x

@check_right:
    LDA buttons
    AND #BUTTON_RIGHT
    BEQ @done
    LDA player_x
    CMP #232                    ; Right boundary (256 - 16 - 8)
    BCS @done
    INC player_x

@done:
    RTS

; ------------------------------------------------------------------------------
; Enable Rendering & Display
; ------------------------------------------------------------------------------
enable_rendering:
    ; Reset PPU scroll position to (0, 0)
    LDA PPU_STATUS
    LDA #$00
    STA PPU_SCROLL
    STA PPU_SCROLL

    ; Configure PPU Control ($2000):
    ; - Enable NMI ($80)
    ; - 8x8 Sprites ($00)
    ; - Background Pattern Table: $1000 ($10)
    ; - Sprite Pattern Table: $0000 ($00)
    ; - Nametable 0: $2000 ($00)
    LDA #%10010000              ; $90
    STA PPU_CTRL

    ; Configure PPU Mask ($2001):
    ; - Show Background ($08)
    ; - Show Sprites ($10)
    ; - Show Background in leftmost 8px ($02)
    ; - Show Sprites in leftmost 8px ($04)
    LDA #%00011110              ; $1E
    STA PPU_MASK

    RTS

; Message text string (null-terminated)
msg_hello:
    .asciiz "Hello, World!"

; ==============================================================================
; 4. PRG Bank 1 ($A000 - $BFFF) - Graphics Assets & Palettes
; ==============================================================================
.bank 1

font_pal:
    .incpal "data/font.png", 4

sprite_pal:
    .incpal "data/sprites.png", 4

sprite_chr:
    .incchr "data/sprites.png"
sprite_chr_end:

font_chr:
    .incchr "data/font.png"
font_chr_end:

; ==============================================================================
; 5. PRG Bank 63 ($E000 - $FFFF) - Fixed System Bank & Vectors
; ==============================================================================
.bank 63

reset_handler:
    SEI                         ; Disable IRQs
    CLD                         ; Clear decimal mode
    LDX #$FF
    TXS                         ; Initialize stack pointer to $FF

    INX                         ; X = 0
    STX PPU_CTRL                ; Disable NMI
    STX PPU_MASK                ; Disable rendering
    STX APU_STATUS              ; Disable APU audio channels
    STX $4010                   ; Disable DMC IRQs

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

    ; 2nd VBLANK wait (PPU is now fully ready for registers & VRAM)
:   BIT PPU_STATUS
    BPL :-

    ; Explicitly map PRG Bank 0 to $8000-$9FFF before jumping to main
    LDA #$06                    ; Select PRG R6 ($8000-$9FFF)
    STA MMC3_BANK_SELECT
    LDA #$00                    ; Select PRG Bank 0
    STA MMC3_BANK_DATA

    ; Jump to Bank 0 Main entry point
    JMP main

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

    ; Reset PPU scroll latch & coordinates
    LDA #$00
    STA PPU_SCROLL
    STA PPU_SCROLL

    ; Signal frame completion to main loop
    LDA #$01
    STA nmi_ready

    PLA
    TAY                         ; Restore Y Register
    PLA
    TAX                         ; Restore X Register
    PLA                         ; Restore Accumulator
    RTI

irq_handler:
    RTI
