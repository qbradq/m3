package mmc

bank 63

// =============================================================================
// m3 Standard Library - MMC3 Memory Management & Bank Control (lib/mmc.m3)
// =============================================================================

// MMC3 Mapper 4 Registers
define (
    MMC3_BANK_SELECT $8000
    MMC3_BANK_DATA   $8001
    MMC3_MIRRORING   $A000
    MMC3_WRAM_PROT   $A001
    MMC3_IRQ_LATCH   $C000
    MMC3_IRQ_RELOAD  $C001
    MMC3_IRQ_DISABLE $E000
    MMC3_IRQ_ENABLE  $E001
)

// Data bank stack (8 levels deep) and stack pointer in RAM/ZP
var (
    data_bank_stack uint8[8] ram
    data_bank_sp    uint8    zp
)

// PushDataBank pushes bank number n onto the data bank stack and switches
// the MMC3 $8000-$9FFF PRG window (Register 6) to it.
//
// Fastcall Parameters (m3 ABI):
//   n: Bank index in Accumulator (A)
func PushDataBank(n uint8) {
    asm {
        LDX _mmc_data_bank_sp
        CPX #8
        BCS @skip_push
        STA _mmc_data_bank_stack, X
        INX
        STX _mmc_data_bank_sp
    @skip_push:
        PHA
        LDA #$06
        STA $8000
        PLA
        STA $8001
    }
}

// PopDataBank removes the top data bank number from the stack and restores
// the MMC3 $8000-$9FFF PRG window (Register 6) to the new top of stack.
// If the stack becomes empty, defaults to bank 0.
func PopDataBank() {
    asm {
        LDX _mmc_data_bank_sp
        BEQ @empty_stack
        DEX
        STX _mmc_data_bank_sp
        BEQ @empty_stack
        DEX
        LDA _mmc_data_bank_stack, X
        JMP @switch_bank
    @empty_stack:
        LDA #$00
    @switch_bank:
        PHA
        LDA #$06
        STA $8000
        PLA
        STA $8001
    }
}
