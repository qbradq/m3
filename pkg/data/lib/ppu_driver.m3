package ppu_driver

bank 63

// =============================================================================
// m3 Standard Library - PPU Display List VBlank Driver (lib/ppu_driver.m3)
// =============================================================================

// PPU Hardware Register Base Addresses
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
    asm {
        LDA #$00
        STA _ppu_driver_list_len
        STA _ppu_driver_list_buf
    }
}

// PushHorizontal buffers a command to copy len bytes from src to PPU RAM
// starting at dest with horizontal (+1) auto-increment during the next VBlank.
//
// Fastcall / Memory Parameters:
//   src:  Source address pointer (_ppu_driver_push_src in ZP)
//   dest: Target PPU VRAM address (_ppu_driver_push_dst in ZP)
//   len:  Number of bytes to copy (_ppu_driver_push_len in ZP)
func PushHorizontal(src *uint8[], dest uint16, len uint8) {
    asm {
        ; Check if buffer has enough space: list_len + 4 + push_len <= 128
        LDA _ppu_driver_list_len
        CLC
        ADC #4
        ADC _ppu_driver_push_len
        BCS @skip_push
        CMP #129
        BCS @skip_push

        LDX _ppu_driver_list_len

        ; 1. Command ID (CMD_HORIZ)
        LDA #$01
        STA _ppu_driver_list_buf, X
        INX

        ; 2. Destination PPU High Byte
        LDA _ppu_driver_push_dst+1
        STA _ppu_driver_list_buf, X
        INX

        ; 3. Destination PPU Low Byte
        LDA _ppu_driver_push_dst
        STA _ppu_driver_list_buf, X
        INX

        ; 4. Payload Length
        LDA _ppu_driver_push_len
        STA _ppu_driver_list_buf, X
        INX

        ; 5. Copy payload bytes from push_src
        LDY #$00
        LDA _ppu_driver_push_len
        BEQ @end_payload
    @payload_loop:
        LDA (_ppu_driver_push_src), Y
        STA _ppu_driver_list_buf, X
        INX
        INY
        CPY _ppu_driver_push_len
        BNE @payload_loop

    @end_payload:
        ; Append CMD_END at current offset
        LDA #$00
        STA _ppu_driver_list_buf, X
        STX _ppu_driver_list_len

    @skip_push:
    }
}

// PushVertical buffers a command to copy len bytes from src to PPU RAM
// starting at dest with vertical (+32) auto-increment during the next VBlank.
//
// Fastcall / Memory Parameters:
//   src:  Source address pointer (_ppu_driver_push_src in ZP)
//   dest: Target PPU VRAM address (_ppu_driver_push_dst in ZP)
//   len:  Number of bytes to copy (_ppu_driver_push_len in ZP)
func PushVertical(src *uint8[], dest uint16, len uint8) {
    asm {
        ; Check if buffer has enough space: list_len + 4 + push_len <= 128
        LDA _ppu_driver_list_len
        CLC
        ADC #4
        ADC _ppu_driver_push_len
        BCS @skip_push
        CMP #129
        BCS @skip_push

        LDX _ppu_driver_list_len

        ; 1. Command ID (CMD_VERT)
        LDA #$02
        STA _ppu_driver_list_buf, X
        INX

        ; 2. Destination PPU High Byte
        LDA _ppu_driver_push_dst+1
        STA _ppu_driver_list_buf, X
        INX

        ; 3. Destination PPU Low Byte
        LDA _ppu_driver_push_dst
        STA _ppu_driver_list_buf, X
        INX

        ; 4. Payload Length
        LDA _ppu_driver_push_len
        STA _ppu_driver_list_buf, X
        INX

        ; 5. Copy payload bytes from push_src
        LDY #$00
        LDA _ppu_driver_push_len
        BEQ @end_payload
    @payload_loop:
        LDA (_ppu_driver_push_src), Y
        STA _ppu_driver_list_buf, X
        INX
        INY
        CPY _ppu_driver_push_len
        BNE @payload_loop

    @end_payload:
        ; Append CMD_END at current offset
        LDA #$00
        STA _ppu_driver_list_buf, X
        STX _ppu_driver_list_len

    @skip_push:
    }
}

// PushByte buffers a single byte patch to PPU RAM at dest during the next VBlank.
//
// Fastcall / Memory Parameters:
//   val:  Single byte value to write (_ppu_driver_push_val in ZP)
//   dest: Target PPU VRAM address (_ppu_driver_push_dst in ZP)
func PushByte(val uint8, dest uint16) {
    asm {
        ; Check if buffer has enough space: list_len + 4 <= 128
        LDA _ppu_driver_list_len
        CLC
        ADC #4
        BCS @skip_push
        CMP #129
        BCS @skip_push

        LDX _ppu_driver_list_len

        ; 1. Command ID (CMD_BYTE)
        LDA #$03
        STA _ppu_driver_list_buf, X
        INX

        ; 2. Destination PPU High Byte
        LDA _ppu_driver_push_dst+1
        STA _ppu_driver_list_buf, X
        INX

        ; 3. Destination PPU Low Byte
        LDA _ppu_driver_push_dst
        STA _ppu_driver_list_buf, X
        INX

        ; 4. Single byte value
        LDA _ppu_driver_push_val
        STA _ppu_driver_list_buf, X
        INX

        ; Append CMD_END at current offset
        LDA #$00
        STA _ppu_driver_list_buf, X
        STX _ppu_driver_list_len

    @skip_push:
    }
}

// PushPalette buffers a 32-byte palette update from src to PPU palette
// RAM ($3F00) during the next VBlank.
//
// Fastcall / Memory Parameters:
//   src: Source address pointer (_ppu_driver_push_src in ZP)
func PushPalette(src *uint8[32]) {
    asm {
        ; Check if buffer has enough space: list_len + 4 + 32 <= 128
        LDA _ppu_driver_list_len
        CLC
        ADC #36
        BCS @skip_push
        CMP #129
        BCS @skip_push

        LDX _ppu_driver_list_len

        ; 1. Command ID (CMD_HORIZ)
        LDA #$01
        STA _ppu_driver_list_buf, X
        INX

        ; 2. Destination PPU High Byte ($3F)
        LDA #$3F
        STA _ppu_driver_list_buf, X
        INX

        ; 3. Destination PPU Low Byte ($00)
        LDA #$00
        STA _ppu_driver_list_buf, X
        INX

        ; 4. Payload Length (32)
        LDA #32
        STA _ppu_driver_list_buf, X
        INX

        ; 5. Copy 32 payload bytes from push_src
        LDY #$00
    @payload_loop:
        LDA (_ppu_driver_push_src), Y
        STA _ppu_driver_list_buf, X
        INX
        INY
        CPY #32
        BNE @payload_loop

        ; Append CMD_END at current offset
        LDA #$00
        STA _ppu_driver_list_buf, X
        STX _ppu_driver_list_len

    @skip_push:
    }
}

// Process iterates over all buffered display list commands, executes the PPU
// transfers, and clears the display list. Must be called during VBlank from the
// NMI handler with PPU enabled.
func Process() {
    asm {
        LDA _ppu_driver_list_len
        BEQ @done

        LDX #$00
    @cmd_loop:
        LDA _ppu_driver_list_buf, X
        BEQ @done
        INX
        CMP #$01
        BEQ @exec_horiz
        CMP #$02
        BEQ @exec_vert
        CMP #$03
        BEQ @exec_byte
        JMP @done

    @exec_horiz:
        ; PPU_CTRL with +1 increment
        LDA #%10001000
        STA $2000

        ; Set PPU destination address
        LDA _ppu_driver_list_buf, X
        STA $2006
        INX
        LDA _ppu_driver_list_buf, X
        STA $2006
        INX

        ; Load length
        LDY _ppu_driver_list_buf, X
        INX
    @horiz_loop:
        LDA _ppu_driver_list_buf, X
        STA $2007
        INX
        DEY
        BNE @horiz_loop
        JMP @cmd_loop

    @exec_vert:
        ; PPU_CTRL with +32 increment
        LDA #%10001100
        STA $2000

        ; Set PPU destination address
        LDA _ppu_driver_list_buf, X
        STA $2006
        INX
        LDA _ppu_driver_list_buf, X
        STA $2006
        INX

        ; Load length
        LDY _ppu_driver_list_buf, X
        INX
    @vert_loop:
        LDA _ppu_driver_list_buf, X
        STA $2007
        INX
        DEY
        BNE @vert_loop
        JMP @cmd_loop

    @exec_byte:
        ; PPU_CTRL with +1 increment
        LDA #%10001000
        STA $2000

        ; Set PPU destination address
        LDA _ppu_driver_list_buf, X
        STA $2006
        INX
        LDA _ppu_driver_list_buf, X
        STA $2006
        INX

        ; Write byte
        LDA _ppu_driver_list_buf, X
        STA $2007
        INX
        JMP @cmd_loop

    @done:
        ; Restore default PPU_CTRL (+1 increment, NMI enabled)
        LDA #%10001000
        STA $2000

        ; Reset PPU scroll ($00, $00)
        LDA #$00
        STA $2005
        STA $2005

        ; Reset display list
        STA _ppu_driver_list_len
        STA _ppu_driver_list_buf
    }
}
