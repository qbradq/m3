; m3 NES Assembler Demo
.export main, reset_handler
.import play_sound

PPU_CTRL = $2000
PPU_MASK = $2001

.bank 0
main:
    LDA #$00
    STA PPU_CTRL
    STA PPU_MASK
    JSR play_sound

@spin:
    JSR wait_vblank
    JMP @spin

wait_vblank:
:   BIT $2002
    BPL :-
    RTS

.bank 63
reset_handler:
    SEI
    CLD
    LDX #$FF
    TXS
    JMP main
