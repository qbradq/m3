package memory

// =============================================================================
// m3 Standard Library - Memory Operations (lib/memory.m3)
// =============================================================================

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
//   src: Source address pointer (_memory_src_ptr in ZP)
//   dst: Destination address pointer (_memory_dst_ptr in ZP)
//   len: Number of bytes to copy (_memory_len_cnt in ZP)
func Copy(src, dst *uint8[], len uint16) {
    asm {
        ; Check if high byte of length > 0
        LDX _memory_len_cnt+1
        BEQ @copy_remainder

        ; Copy 256-byte full pages
    @page_loop:
        LDY #$00
    @page_byte:
        LDA (_memory_src_ptr), Y
        STA (_memory_dst_ptr), Y
        INY
        BNE @page_byte

        INC _memory_src_ptr+1
        INC _memory_dst_ptr+1
        DEX
        BNE @page_loop

    @copy_remainder:
        LDX _memory_len_cnt
        BEQ @done
        LDY #$00
    @rem_byte:
        LDA (_memory_src_ptr), Y
        STA (_memory_dst_ptr), Y
        INY
        DEX
        BNE @rem_byte

    @done:
    }
}
