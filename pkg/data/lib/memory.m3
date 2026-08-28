package memory

bank 63

// =============================================================================
// m3 Standard Library - Memory Operations (lib/memory.m3)
// =============================================================================

// Copy copies len bytes from src to dst. The behavior is undefined if src
// overlaps dst.
//
// Fastcall Parameters (3-byte register + excess ZP):
//   src: Registers A (low), X (high)
//   dst: __leaf_param0 (low), __leaf_param1 (high)
//   len: __leaf_param2 (low), __leaf_param3 (high)
func Copy(src, dst *uint8[], len uint16) {
    asm {
        STA __leaf_param4
        STX __leaf_param5

        ; Check if high byte of length > 0
        LDX __leaf_param3
        BEQ @copy_remainder

        ; Copy 256-byte full pages
    @page_loop:
        LDY #$00
    @page_byte:
        LDA (__leaf_param4), Y
        STA (__leaf_param0), Y
        INY
        BNE @page_byte

        INC __leaf_param5
        INC __leaf_param1
        DEX
        BNE @page_loop

    @copy_remainder:
        LDX __leaf_param2
        BEQ @done
        LDY #$00
    @rem_byte:
        LDA (__leaf_param4), Y
        STA (__leaf_param0), Y
        INY
        DEX
        BNE @rem_byte

    @done:
    }
}
