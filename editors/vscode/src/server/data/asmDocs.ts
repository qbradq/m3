import { CompletionItemKind, InsertTextFormat } from 'vscode-languageserver/node';
import { DocEntry } from './m3Docs';

export interface InstructionDocEntry extends DocEntry {
  flags: string;
  modes: string[];
}

export const ASM_INSTRUCTIONS: Record<string, InstructionDocEntry> = {
  ADC: {
    name: 'ADC',
    kind: CompletionItemKind.Keyword,
    detail: 'ADC (Add with Carry)',
    flags: 'N V Z C',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)', 'Absolute, Y ($nnnn,Y)', 'Indexed Indirect (($nn,X))', 'Indirect Indexed (($nn),Y)'],
    documentation: `**ADC** adds the contents of a memory value along with the carry flag to the accumulator, storing the result in the accumulator.

### Affected Flags:
- **N**: Set if bit 7 of result is set (negative).
- **V**: Set if sign bit overflow occurred.
- **Z**: Set if result is zero (\`$00\`).
- **C**: Set if an unsigned overflow occurred (result > 255).

### Syntax:
\`\`\`assembly
ADC #$10        ; Add immediate value
ADC player_x    ; Add zero page variable
ADC (ptr), Y    ; Add indirect indexed
\`\`\``,
  },
  AND: {
    name: 'AND',
    kind: CompletionItemKind.Keyword,
    detail: 'AND (Bitwise Logical AND)',
    flags: 'N Z',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)', 'Absolute, Y ($nnnn,Y)', 'Indexed Indirect (($nn,X))', 'Indirect Indexed (($nn),Y)'],
    documentation: `**AND** performs a bitwise logical AND between the accumulator and memory operand, storing the result in the accumulator.

### Affected Flags:
- **N**: Set if bit 7 of result is set.
- **Z**: Set if result is zero.`,
  },
  ASL: {
    name: 'ASL',
    kind: CompletionItemKind.Keyword,
    detail: 'ASL (Arithmetic Shift Left)',
    flags: 'N Z C',
    modes: ['Accumulator (A)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)'],
    documentation: `**ASL** shifts all bits in the accumulator or memory left by 1 bit. Bit 7 is shifted into the Carry flag, and bit 0 is cleared to 0.

### Affected Flags:
- **N**: Set if bit 7 of result is set.
- **Z**: Set if result is zero.
- **C**: Receives previous bit 7.`,
  },
  BCC: {
    name: 'BCC',
    kind: CompletionItemKind.Keyword,
    detail: 'BCC (Branch if Carry Clear)',
    flags: 'None',
    modes: ['Relative (label)'],
    documentation: `**BCC** branches to the target address if the Carry flag is clear (\`C = 0\`). Often used after \`CMP\` for branch if less than (\`<\`).`,
  },
  BCS: {
    name: 'BCS',
    kind: CompletionItemKind.Keyword,
    detail: 'BCS (Branch if Carry Set)',
    flags: 'None',
    modes: ['Relative (label)'],
    documentation: `**BCS** branches to the target address if the Carry flag is set (\`C = 1\`). Often used after \`CMP\` for branch if greater than or equal (\`>=\`).`,
  },
  BEQ: {
    name: 'BEQ',
    kind: CompletionItemKind.Keyword,
    detail: 'BEQ (Branch if Equal / Zero Set)',
    flags: 'None',
    modes: ['Relative (label)'],
    documentation: `**BEQ** branches to the target address if the Zero flag is set (\`Z = 1\`, value is zero or operands matched).`,
  },
  BIT: {
    name: 'BIT',
    kind: CompletionItemKind.Keyword,
    detail: 'BIT (Bit Test)',
    flags: 'N V Z',
    modes: ['Zero Page ($nn)', 'Absolute ($nnnn)'],
    documentation: `**BIT** performs a bitwise test between the accumulator and a memory value:
- **Z**: Set if \`A & operand == 0\`.
- **N**: Directly copies bit 7 of operand.
- **V**: Directly copies bit 6 of operand.

Commonly used to poll hardware registers (e.g., \`BIT $2002\` for PPU VBlank).`,
  },
  BMI: {
    name: 'BMI',
    kind: CompletionItemKind.Keyword,
    detail: 'BMI (Branch if Minus / Negative Set)',
    flags: 'None',
    modes: ['Relative (label)'],
    documentation: `**BMI** branches to the target address if the Negative flag is set (\`N = 1\`).`,
  },
  BNE: {
    name: 'BNE',
    kind: CompletionItemKind.Keyword,
    detail: 'BNE (Branch if Not Equal / Zero Clear)',
    flags: 'None',
    modes: ['Relative (label)'],
    documentation: `**BNE** branches to the target address if the Zero flag is clear (\`Z = 0\`, value is non-zero or comparison differed).`,
  },
  BPL: {
    name: 'BPL',
    kind: CompletionItemKind.Keyword,
    detail: 'BPL (Branch if Positive / Negative Clear)',
    flags: 'None',
    modes: ['Relative (label)'],
    documentation: `**BPL** branches to the target address if the Negative flag is clear (\`N = 0\`).`,
  },
  BRK: {
    name: 'BRK',
    kind: CompletionItemKind.Keyword,
    detail: 'BRK (Force Break / Software Interrupt)',
    flags: 'B I',
    modes: ['Implied'],
    documentation: `**BRK** forces generation of an interrupt request. The program counter and processor status (with Break bit set) are pushed to the stack, and execution vectors to \`$FFFE\`.`,
  },
  BVC: {
    name: 'BVC',
    kind: CompletionItemKind.Keyword,
    detail: 'BVC (Branch if Overflow Clear)',
    flags: 'None',
    modes: ['Relative (label)'],
    documentation: `**BVC** branches if the Overflow flag is clear (\`V = 0\`).`,
  },
  BVS: {
    name: 'BVS',
    kind: CompletionItemKind.Keyword,
    detail: 'BVS (Branch if Overflow Set)',
    flags: 'None',
    modes: ['Relative (label)'],
    documentation: `**BVS** branches if the Overflow flag is set (\`V = 1\`).`,
  },
  CLC: {
    name: 'CLC',
    kind: CompletionItemKind.Keyword,
    detail: 'CLC (Clear Carry Flag)',
    flags: 'C (0)',
    modes: ['Implied'],
    documentation: `**CLC** clears the Carry flag (\`C = 0\`). Typically executed before an \`ADC\` addition.`,
  },
  CLD: {
    name: 'CLD',
    kind: CompletionItemKind.Keyword,
    detail: 'CLD (Clear Decimal Mode)',
    flags: 'D (0)',
    modes: ['Implied'],
    documentation: `**CLD** clears the Decimal mode flag (\`D = 0\`). Recommended during system reset on NES (which disables BCD hardware in the 2A03).`,
  },
  CLI: {
    name: 'CLI',
    kind: CompletionItemKind.Keyword,
    detail: 'CLI (Clear Interrupt Disable)',
    flags: 'I (0)',
    modes: ['Implied'],
    documentation: `**CLI** clears the Interrupt disable flag (\`I = 0\`), enabling IRQ maskable interrupts.`,
  },
  CLV: {
    name: 'CLV',
    kind: CompletionItemKind.Keyword,
    detail: 'CLV (Clear Overflow Flag)',
    flags: 'V (0)',
    modes: ['Implied'],
    documentation: `**CLV** clears the Overflow flag (\`V = 0\`).`,
  },
  CMP: {
    name: 'CMP',
    kind: CompletionItemKind.Keyword,
    detail: 'CMP (Compare Accumulator)',
    flags: 'N Z C',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)', 'Absolute, Y ($nnnn,Y)', 'Indexed Indirect (($nn,X))', 'Indirect Indexed (($nn),Y)'],
    documentation: `**CMP** compares the accumulator with a memory value by performing \`A - memory\`:
- **Z**: Set if \`A == operand\`.
- **C**: Set if \`A >= operand\` (unsigned).
- **N**: Set if bit 7 of \`(A - operand)\` is set.`,
  },
  CPX: {
    name: 'CPX',
    kind: CompletionItemKind.Keyword,
    detail: 'CPX (Compare X Register)',
    flags: 'N Z C',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Absolute ($nnnn)'],
    documentation: `**CPX** compares the X register with a memory value (\`X - memory\`).`,
  },
  CPY: {
    name: 'CPY',
    kind: CompletionItemKind.Keyword,
    detail: 'CPY (Compare Y Register)',
    flags: 'N Z C',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Absolute ($nnnn)'],
    documentation: `**CPY** compares the Y register with a memory value (\`Y - memory\`).`,
  },
  DEC: {
    name: 'DEC',
    kind: CompletionItemKind.Keyword,
    detail: 'DEC (Decrement Memory)',
    flags: 'N Z',
    modes: ['Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)'],
    documentation: `**DEC** decrements the value at specified memory location by 1.

### Affected Flags:
- **N**: Set if bit 7 is set.
- **Z**: Set if result is zero.`,
  },
  DEX: {
    name: 'DEX',
    kind: CompletionItemKind.Keyword,
    detail: 'DEX (Decrement X Register)',
    flags: 'N Z',
    modes: ['Implied'],
    documentation: `**DEX** decrements the X register by 1.`,
  },
  DEY: {
    name: 'DEY',
    kind: CompletionItemKind.Keyword,
    detail: 'DEY (Decrement Y Register)',
    flags: 'N Z',
    modes: ['Implied'],
    documentation: `**DEY** decrements the Y register by 1.`,
  },
  EOR: {
    name: 'EOR',
    kind: CompletionItemKind.Keyword,
    detail: 'EOR (Exclusive OR Accumulator)',
    flags: 'N Z',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)', 'Absolute, Y ($nnnn,Y)', 'Indexed Indirect (($nn,X))', 'Indirect Indexed (($nn),Y)'],
    documentation: `**EOR** performs a bitwise Exclusive-OR (XOR) on the accumulator with a memory value.`,
  },
  INC: {
    name: 'INC',
    kind: CompletionItemKind.Keyword,
    detail: 'INC (Increment Memory)',
    flags: 'N Z',
    modes: ['Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)'],
    documentation: `**INC** increments the value at specified memory location by 1.`,
  },
  INX: {
    name: 'INX',
    kind: CompletionItemKind.Keyword,
    detail: 'INX (Increment X Register)',
    flags: 'N Z',
    modes: ['Implied'],
    documentation: `**INX** increments the X register by 1.`,
  },
  INY: {
    name: 'INY',
    kind: CompletionItemKind.Keyword,
    detail: 'INY (Increment Y Register)',
    flags: 'N Z',
    modes: ['Implied'],
    documentation: `**INY** increments the Y register by 1.`,
  },
  JMP: {
    name: 'JMP',
    kind: CompletionItemKind.Keyword,
    detail: 'JMP (Jump to Address)',
    flags: 'None',
    modes: ['Absolute ($nnnn)', 'Indirect (($nnnn))'],
    documentation: `**JMP** sets the Program Counter to the target address.

### Syntax:
\`\`\`assembly
JMP main_loop
JMP (jump_table)
\`\`\``,
  },
  JSR: {
    name: 'JSR',
    kind: CompletionItemKind.Keyword,
    detail: 'JSR (Jump to Subroutine)',
    flags: 'None',
    modes: ['Absolute ($nnnn)'],
    documentation: `**JSR** pushes the return address (PC + 2) onto the stack and transfers execution to the target subroutine.`,
  },
  LDA: {
    name: 'LDA',
    kind: CompletionItemKind.Keyword,
    detail: 'LDA (Load Accumulator)',
    flags: 'N Z',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)', 'Absolute, Y ($nnnn,Y)', 'Indexed Indirect (($nn,X))', 'Indirect Indexed (($nn),Y)'],
    documentation: `**LDA** loads a byte of memory or immediate value into the accumulator.

### Affected Flags:
- **N**: Set if bit 7 of loaded byte is set.
- **Z**: Set if loaded byte is zero (\`$00\`).`,
  },
  LDX: {
    name: 'LDX',
    kind: CompletionItemKind.Keyword,
    detail: 'LDX (Load X Register)',
    flags: 'N Z',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Zero Page, Y ($nn,Y)', 'Absolute ($nnnn)', 'Absolute, Y ($nnnn,Y)'],
    documentation: `**LDX** loads a byte into the X index register.`,
  },
  LDY: {
    name: 'LDY',
    kind: CompletionItemKind.Keyword,
    detail: 'LDY (Load Y Register)',
    flags: 'N Z',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)'],
    documentation: `**LDY** loads a byte into the Y index register.`,
  },
  LSR: {
    name: 'LSR',
    kind: CompletionItemKind.Keyword,
    detail: 'LSR (Logical Shift Right)',
    flags: 'N (0) Z C',
    modes: ['Accumulator (A)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)'],
    documentation: `**LSR** shifts all bits right by 1. Bit 0 is shifted into the Carry flag, and bit 7 is set to 0.`,
  },
  NOP: {
    name: 'NOP',
    kind: CompletionItemKind.Keyword,
    detail: 'NOP (No Operation)',
    flags: 'None',
    modes: ['Implied'],
    documentation: `**NOP** performs no operation. Consumes 1 byte and 2 CPU cycles.`,
  },
  ORA: {
    name: 'ORA',
    kind: CompletionItemKind.Keyword,
    detail: 'ORA (Bitwise Logical OR Accumulator)',
    flags: 'N Z',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)', 'Absolute, Y ($nnnn,Y)', 'Indexed Indirect (($nn,X))', 'Indirect Indexed (($nn),Y)'],
    documentation: `**ORA** performs a bitwise logical inclusive OR between the accumulator and memory operand.`,
  },
  PHA: {
    name: 'PHA',
    kind: CompletionItemKind.Keyword,
    detail: 'PHA (Push Accumulator to Stack)',
    flags: 'None',
    modes: ['Implied'],
    documentation: `**PHA** pushes the current contents of the accumulator onto the hardware stack (\`$0100 - $01FF\`).`,
  },
  PHP: {
    name: 'PHP',
    kind: CompletionItemKind.Keyword,
    detail: 'PHP (Push Processor Status to Stack)',
    flags: 'None',
    modes: ['Implied'],
    documentation: `**PHP** pushes the processor status register (flags) onto the stack with the Break and unused bits set.`,
  },
  PLA: {
    name: 'PLA',
    kind: CompletionItemKind.Keyword,
    detail: 'PLA (Pull Accumulator from Stack)',
    flags: 'N Z',
    modes: ['Implied'],
    documentation: `**PLA** pulls a byte from the stack into the accumulator. Sets N and Z flags based on the pulled value.`,
  },
  PLP: {
    name: 'PLP',
    kind: CompletionItemKind.Keyword,
    detail: 'PLP (Pull Processor Status from Stack)',
    flags: 'All Flags (Restored)',
    modes: ['Implied'],
    documentation: `**PLP** pulls a byte from the stack and restores the processor status register flags.`,
  },
  ROL: {
    name: 'ROL',
    kind: CompletionItemKind.Keyword,
    detail: 'ROL (Rotate Left through Carry)',
    flags: 'N Z C',
    modes: ['Accumulator (A)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)'],
    documentation: `**ROL** rotates bits left: bit 7 enters the Carry flag, and the previous Carry flag enters bit 0.`,
  },
  ROR: {
    name: 'ROR',
    kind: CompletionItemKind.Keyword,
    detail: 'ROR (Rotate Right through Carry)',
    flags: 'N Z C',
    modes: ['Accumulator (A)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)'],
    documentation: `**ROR** rotates bits right: bit 0 enters the Carry flag, and the previous Carry flag enters bit 7.`,
  },
  RTI: {
    name: 'RTI',
    kind: CompletionItemKind.Keyword,
    detail: 'RTI (Return from Interrupt)',
    flags: 'All Flags (Restored)',
    modes: ['Implied'],
    documentation: `**RTI** restores processor flags and Program Counter from the stack. Used at the end of NMI and IRQ handlers.`,
  },
  RTS: {
    name: 'RTS',
    kind: CompletionItemKind.Keyword,
    detail: 'RTS (Return from Subroutine)',
    flags: 'None',
    modes: ['Implied'],
    documentation: `**RTS** pulls the return address from the stack and resumes execution after the matching \`JSR\`.`,
  },
  SBC: {
    name: 'SBC',
    kind: CompletionItemKind.Keyword,
    detail: 'SBC (Subtract with Carry / Borrow)',
    flags: 'N V Z C',
    modes: ['Immediate (#$nn)', 'Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)', 'Absolute, Y ($nnnn,Y)', 'Indexed Indirect (($nn,X))', 'Indirect Indexed (($nn),Y)'],
    documentation: `**SBC** subtracts memory operand and borrow (\`1 - C\`) from accumulator (\`A = A - memory - (1 - C)\`).

Must be preceded by \`SEC\` for standard subtraction without borrow.`,
  },
  SEC: {
    name: 'SEC',
    kind: CompletionItemKind.Keyword,
    detail: 'SEC (Set Carry Flag)',
    flags: 'C (1)',
    modes: ['Implied'],
    documentation: `**SEC** sets the Carry flag (\`C = 1\`). Executed prior to \`SBC\` subtraction.`,
  },
  SED: {
    name: 'SED',
    kind: CompletionItemKind.Keyword,
    detail: 'SED (Set Decimal Flag)',
    flags: 'D (1)',
    modes: ['Implied'],
    documentation: `**SED** sets the Decimal mode flag (\`D = 1\`).`,
  },
  SEI: {
    name: 'SEI',
    kind: CompletionItemKind.Keyword,
    detail: 'SEI (Set Interrupt Disable)',
    flags: 'I (1)',
    modes: ['Implied'],
    documentation: `**SEI** sets the Interrupt disable flag (\`I = 1\`), preventing maskable IRQ interrupts.`,
  },
  STA: {
    name: 'STA',
    kind: CompletionItemKind.Keyword,
    detail: 'STA (Store Accumulator in Memory)',
    flags: 'None',
    modes: ['Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)', 'Absolute, X ($nnnn,X)', 'Absolute, Y ($nnnn,Y)', 'Indexed Indirect (($nn,X))', 'Indirect Indexed (($nn),Y)'],
    documentation: `**STA** stores the contents of the accumulator into the specified memory address.`,
  },
  STX: {
    name: 'STX',
    kind: CompletionItemKind.Keyword,
    detail: 'STX (Store X Register in Memory)',
    flags: 'None',
    modes: ['Zero Page ($nn)', 'Zero Page, Y ($nn,Y)', 'Absolute ($nnnn)'],
    documentation: `**STX** stores the contents of the X register into the specified memory address.`,
  },
  STY: {
    name: 'STY',
    kind: CompletionItemKind.Keyword,
    detail: 'STY (Store Y Register in Memory)',
    flags: 'None',
    modes: ['Zero Page ($nn)', 'Zero Page, X ($nn,X)', 'Absolute ($nnnn)'],
    documentation: `**STY** stores the contents of the Y register into the specified memory address.`,
  },
  TAX: {
    name: 'TAX',
    kind: CompletionItemKind.Keyword,
    detail: 'TAX (Transfer Accumulator to X)',
    flags: 'N Z',
    modes: ['Implied'],
    documentation: `**TAX** copies the accumulator into the X register (\`X = A\`).`,
  },
  TAY: {
    name: 'TAY',
    kind: CompletionItemKind.Keyword,
    detail: 'TAY (Transfer Accumulator to Y)',
    flags: 'N Z',
    modes: ['Implied'],
    documentation: `**TAY** copies the accumulator into the Y register (\`Y = A\`).`,
  },
  TSX: {
    name: 'TSX',
    kind: CompletionItemKind.Keyword,
    detail: 'TSX (Transfer Stack Pointer to X)',
    flags: 'N Z',
    modes: ['Implied'],
    documentation: `**TSX** copies the hardware stack pointer into the X register (\`X = S\`).`,
  },
  TXA: {
    name: 'TXA',
    kind: CompletionItemKind.Keyword,
    detail: 'TXA (Transfer X to Accumulator)',
    flags: 'N Z',
    modes: ['Implied'],
    documentation: `**TXA** copies the X register into the accumulator (\`A = X\`).`,
  },
  TXS: {
    name: 'TXS',
    kind: CompletionItemKind.Keyword,
    detail: 'TXS (Transfer X to Stack Pointer)',
    flags: 'None',
    modes: ['Implied'],
    documentation: `**TXS** copies the X register into the hardware stack pointer (\`S = X\`).`,
  },
  TYA: {
    name: 'TYA',
    kind: CompletionItemKind.Keyword,
    detail: 'TYA (Transfer Y to Accumulator)',
    flags: 'N Z',
    modes: ['Implied'],
    documentation: `**TYA** copies the Y register into the accumulator (\`A = Y\`).`,
  },
};

export const ASM_DIRECTIVES: Record<string, DocEntry> = {
  '.bank': {
    name: '.bank',
    kind: CompletionItemKind.Keyword,
    detail: '.bank <index | auto>',
    documentation: `**.bank** informs the assembler which 8KB MMC3 PRG-ROM bank the following symbols and code belong to.

### Syntax:
\`\`\`assembly
.bank 0          ; Fixed bank 0
.bank auto       ; Link-time auto-packing into available PRG bank
.bank 63         ; MMC3 fixed bank ($E000-$FFFF)
\`\`\``,
    snippet: '.bank ${1:0}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.zp': {
    name: '.zp',
    kind: CompletionItemKind.Keyword,
    detail: '.zp [<size>] (Zero Page Segment Switch / Alloc)',
    documentation: `**.zp** (alias **.zeropage**) switches context to the Zero Page memory segment (\`$0000\`-\`$00FF\`), or allocates bytes directly when given a size argument (\`.zp 2\`).`,
  },
  '.zeropage': {
    name: '.zeropage',
    kind: CompletionItemKind.Keyword,
    detail: '.zeropage [<size>] (Zero Page Segment Switch / Alloc)',
    documentation: `**.zeropage** is an alias for **.zp**.`,
  },
  '.ram': {
    name: '.ram',
    kind: CompletionItemKind.Keyword,
    detail: '.ram [<size>] (CPU RAM / BSS Segment Switch / Alloc)',
    documentation: `**.ram** (alias **.bss**) switches context to the CPU RAM segment (\`$0300\`-\`$07FF\`), or allocates bytes directly (\`.ram 64\`).`,
  },
  '.bss': {
    name: '.bss',
    kind: CompletionItemKind.Keyword,
    detail: '.bss [<size>] (Alias for .ram)',
    documentation: `**.bss** is an alias for **.ram**.`,
  },
  '.wram': {
    name: '.wram',
    kind: CompletionItemKind.Keyword,
    detail: '.wram [<size>] (MMC3 Work RAM Segment Switch / Alloc)',
    documentation: `**.wram** (alias **.prgram**, **.sram**) switches context to MMC3 8KB Work RAM (\`$6000\`-\`$7FFF\`), or allocates bytes directly (\`.wram 128\`). Enables battery-backed header flag at link time.`,
  },
  '.byte': {
    name: '.byte',
    kind: CompletionItemKind.Keyword,
    detail: '.byte <val1>, <val2>... (Emit 8-bit Bytes)',
    documentation: `**.byte** (alias **.byt**, **.db**) emits one or more 8-bit byte values or string literals into the output stream.

### Example:
\`\`\`assembly
.byte $01, $02, $03, $FF
.byte "NES", 0
\`\`\``,
    snippet: '.byte ${1:$00}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.byt': {
    name: '.byt',
    kind: CompletionItemKind.Keyword,
    detail: '.byt <val1>, <val2>... (Alias for .byte)',
    documentation: `**.byt** is an alias for **.byte**.`,
  },
  '.db': {
    name: '.db',
    kind: CompletionItemKind.Keyword,
    detail: '.db <val1>, <val2>... (Alias for .byte)',
    documentation: `**.db** is an alias for **.byte**.`,
  },
  '.word': {
    name: '.word',
    kind: CompletionItemKind.Keyword,
    detail: '.word <val1>, <val2>... (Emit 16-bit Words)',
    documentation: `**.word** (alias **.addr**, **.dw**) emits one or more 16-bit little-endian words (low byte first, high byte second).

### Example:
\`\`\`assembly
.word $8000, main, nmi_handler
\`\`\``,
    snippet: '.word ${1:label}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.addr': {
    name: '.addr',
    kind: CompletionItemKind.Keyword,
    detail: '.addr <val1>... (Alias for .word)',
    documentation: `**.addr** is an alias for **.word**.`,
  },
  '.dw': {
    name: '.dw',
    kind: CompletionItemKind.Keyword,
    detail: '.dw <val1>... (Alias for .word)',
    documentation: `**.dw** is an alias for **.word**.`,
  },
  '.dword': {
    name: '.dword',
    kind: CompletionItemKind.Keyword,
    detail: '.dword <val1>... (Emit 32-bit Double Words)',
    documentation: `**.dword** (alias **.dd**) emits one or more 32-bit little-endian values into the output stream.`,
  },
  '.asciiz': {
    name: '.asciiz',
    kind: CompletionItemKind.Keyword,
    detail: '.asciiz "<string>" (Emit Null-Terminated ASCII)',
    documentation: `**.asciiz** (alias **.stringz**) emits an ASCII string literal followed by a null-terminator byte (\`$00\`).`,
    snippet: '.asciiz "${1:text}"',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.res': {
    name: '.res',
    kind: CompletionItemKind.Keyword,
    detail: '.res <count> [, <fill_byte>] (Reserve Bytes)',
    documentation: `**.res** (alias **.reserve**) reserves a specified number of bytes in the current segment, optionally filled with a byte value (default: \`$00\`).

### Example:
\`\`\`assembly
buffer: .res 64
table:  .res 16, $FF
\`\`\``,
    snippet: '.res ${1:1}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.export': {
    name: '.export',
    kind: CompletionItemKind.Keyword,
    detail: '.export <sym1>, <sym2>... (Export Global Symbols)',
    documentation: `**.export** (alias **.global**) exports symbols so they can be referenced by other source files during linking.`,
    snippet: '.export ${1:symbol}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.global': {
    name: '.global',
    kind: CompletionItemKind.Keyword,
    detail: '.global <sym1>... (Alias for .export)',
    documentation: `**.global** is an alias for **.export**.`,
  },
  '.import': {
    name: '.import',
    kind: CompletionItemKind.Keyword,
    detail: '.import <sym1>, <sym2>... (Import External Symbols)',
    documentation: `**.import** declares external symbols defined in other compilation units.`,
    snippet: '.import ${1:symbol}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.importzp': {
    name: '.importzp',
    kind: CompletionItemKind.Keyword,
    detail: '.importzp <sym1>... (Import External Zero Page Symbols)',
    documentation: `**.importzp** declares external symbols residing in Zero Page, enabling Zero Page addressing optimizations.`,
    snippet: '.importzp ${1:symbol}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.proc': {
    name: '.proc',
    kind: CompletionItemKind.Keyword,
    detail: '.proc <name> ... .endproc (Scoped Procedure Block)',
    documentation: `**.proc** defines a scoped procedural block. Symbols defined within the block are local to the procedure unless exported.`,
    snippet: '.proc ${1:name}\n\t${0}\n\tRTS\n.endproc',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.endproc': {
    name: '.endproc',
    kind: CompletionItemKind.Keyword,
    detail: '.endproc (End Procedure Block)',
    documentation: `**.endproc** closes a \`.proc\` procedure block.`,
  },
  '.scope': {
    name: '.scope',
    kind: CompletionItemKind.Keyword,
    detail: '.scope <name> ... .endscope (Lexical Scope)',
    documentation: `**.scope** defines a generic lexical scope for symbols. Sub-symbols can be accessed externally via \`<scope>::<symbol>\`.`,
    snippet: '.scope ${1:name}\n\t${0}\n.endscope',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.endscope': {
    name: '.endscope',
    kind: CompletionItemKind.Keyword,
    detail: '.endscope (End Lexical Scope)',
    documentation: `**.endscope** closes a \`.scope\` block.`,
  },
  '.macro': {
    name: '.macro',
    kind: CompletionItemKind.Keyword,
    detail: '.macro <name> [<param1>, ...] ... .endmacro',
    documentation: `**.macro** defines a parameterized template expansion macro.

### Example:
\`\`\`assembly
.macro set_ppu_addr addr
    LDA #>addr
    STA $2006
    LDA #<addr
    STA $2006
.endmacro
\`\`\``,
    snippet: '.macro ${1:name} ${2:arg}\n\t${0}\n.endmacro',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.endmacro': {
    name: '.endmacro',
    kind: CompletionItemKind.Keyword,
    detail: '.endmacro (End Macro Block)',
    documentation: `**.endmacro** closes a \`.macro\` definition.`,
  },
  '.include': {
    name: '.include',
    kind: CompletionItemKind.Keyword,
    detail: '.include "<path>" (Include Source File)',
    documentation: `**.include** includes another assembly source file inline as if its contents were written in-place.`,
    snippet: '.include "${1:file.inc}"',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.incbin': {
    name: '.incbin',
    kind: CompletionItemKind.Keyword,
    detail: '.incbin "<path>" [, <offset>, <length>] (Include Binary File)',
    documentation: `**.incbin** includes raw binary data directly into the assembled output with optional byte offset and length.`,
    snippet: '.incbin "${1:file.bin}"',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.incchr': {
    name: '.incchr',
    kind: CompletionItemKind.Keyword,
    detail: '.incchr "<image.png>" (Convert Image to NES CHR Tiles)',
    documentation: `**.incchr** converts a PNG image directly into standard NES 2BPP planar CHR tile data (16 bytes per 8x8 pixel tile in row-major order). Image dimensions must be multiples of 8.

### Example:
\`\`\`assembly
.bank 1
font_chr:
    .incchr "assets/font.png"
\`\`\``,
    snippet: '.incchr "${1:assets/tiles.png}"',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.incpal': {
    name: '.incpal',
    kind: CompletionItemKind.Keyword,
    detail: '.incpal "<image.png>" [, <count>] (Extract Palette Bytes)',
    documentation: `**.incpal** extracts palette colors from a PNG image and converts them to NES hardware 2C02 palette index bytes (\`$00\`–\`$3F\`). Optional count defaults to 4 (single sub-palette) or up to 16.`,
    snippet: '.incpal "${1:assets/title.png}", ${2:4}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.define': {
    name: '.define',
    kind: CompletionItemKind.Keyword,
    detail: '.define <name> <expr> (Constant Definition)',
    documentation: `**.define** (alias **.def**) defines an assemble-time constant value.

### Example:
\`\`\`assembly
.define PPU_CTRL $2000
.define SCREEN_WIDTH 256
\`\`\``,
    snippet: '.define ${1:NAME} ${2:$0000}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.def': {
    name: '.def',
    kind: CompletionItemKind.Keyword,
    detail: '.def <name> <expr> (Alias for .define)',
    documentation: `**.def** is an alias for **.define**.`,
  },
  '.set': {
    name: '.set',
    kind: CompletionItemKind.Keyword,
    detail: '<name> .set <expr> (Reassignable Symbol)',
    documentation: `**.set** assigns a reassignable assemble-time value to a symbol.`,
  },
  '.equ': {
    name: '.equ',
    kind: CompletionItemKind.Keyword,
    detail: '<name> .equ <expr> (Constant Assignment)',
    documentation: `**.equ** assigns a constant assemble-time value to a symbol.`,
  },
  '.if': {
    name: '.if',
    kind: CompletionItemKind.Keyword,
    detail: '.if <expr> ... [.else] ... .endif (Conditional Assembly)',
    documentation: `**.if** begins a conditional assembly block evaluated at assemble-time.`,
    snippet: '.if ${1:EXPR}\n\t${0}\n.endif',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  '.ifdef': {
    name: '.ifdef',
    kind: CompletionItemKind.Keyword,
    detail: '.ifdef <symbol> ... .endif',
    documentation: `**.ifdef** compiles the enclosed code if \`<symbol>\` has been defined.`,
  },
  '.ifndef': {
    name: '.ifndef',
    kind: CompletionItemKind.Keyword,
    detail: '.ifndef <symbol> ... .endif',
    documentation: `**.ifndef** compiles the enclosed code if \`<symbol>\` has NOT been defined.`,
  },
  '.elseif': {
    name: '.elseif',
    kind: CompletionItemKind.Keyword,
    detail: '.elseif <expr>',
    documentation: `**.elseif** provides an alternative conditional branch in an \`.if\` block.`,
  },
  '.else': {
    name: '.else',
    kind: CompletionItemKind.Keyword,
    detail: '.else',
    documentation: `**.else** provides the fallback branch in an \`.if\` block.`,
  },
  '.endif': {
    name: '.endif',
    kind: CompletionItemKind.Keyword,
    detail: '.endif',
    documentation: `**.endif** closes a conditional assembly block (\`.if\`, \`.ifdef\`, \`.ifndef\`).`,
  },
};

export const NES_REGISTERS: Record<string, DocEntry> = {
  PPU_CTRL: {
    name: 'PPU_CTRL',
    kind: CompletionItemKind.Constant,
    detail: 'PPU_CTRL = $2000 (PPU Control Register 1)',
    documentation: `**PPU_CTRL ($2000)** (Write-only):
- Bit 7: Execute NMI on VBlank (0 = Disabled, 1 = Enabled)
- Bit 6: PPU Master/Slave (0 = Read backdrop from EXT, 1 = Output color on EXT)
- Bit 5: Sprite size (0 = 8x8, 1 = 8x16)
- Bit 4: Background pattern table address (0 = $0000, 1 = $1000)
- Bit 3: Sprite pattern table address (0 = $0000, 1 = $1000)
- Bit 2: VRAM address increment (0 = add 1 across, 1 = add 32 down)
- Bit 1-0: Base nametable address (0 = $2000, 1 = $2400, 2 = $2800, 3 = $2C00)`,
  },
  PPU_MASK: {
    name: 'PPU_MASK',
    kind: CompletionItemKind.Constant,
    detail: 'PPU_MASK = $2001 (PPU Mask Register / Rendering)',
    documentation: `**PPU_MASK ($2001)** (Write-only):
- Bit 7: Emphasize blue
- Bit 6: Emphasize green
- Bit 5: Emphasize red
- Bit 4: Show sprites (1 = Enabled)
- Bit 3: Show background (1 = Enabled)
- Bit 2: Show sprites in leftmost 8 pixels
- Bit 1: Show background in leftmost 8 pixels
- Bit 0: Grayscale (0 = Color, 1 = Grayscale)`,
  },
  PPU_STATUS: {
    name: 'PPU_STATUS',
    kind: CompletionItemKind.Constant,
    detail: 'PPU_STATUS = $2002 (PPU Status Register)',
    documentation: `**PPU_STATUS ($2002)** (Read-only):
- Bit 7: VBlank flag (1 = in VBlank, cleared upon reading)
- Bit 6: Sprite 0 hit flag
- Bit 5: Sprite overflow flag
- Bit 4-0: Open bus contents`,
  },
  OAM_ADDR: {
    name: 'OAM_ADDR',
    kind: CompletionItemKind.Constant,
    detail: 'OAM_ADDR = $2003 (OAM Address Register)',
    documentation: `**OAM_ADDR ($2003)** (Write-only): Sets byte offset in 256-byte OAM memory.`,
  },
  OAM_DATA: {
    name: 'OAM_DATA',
    kind: CompletionItemKind.Constant,
    detail: 'OAM_DATA = $2004 (OAM Data Port)',
    documentation: `**OAM_DATA ($2004)** (Read/Write): Read or write OAM sprite data.`,
  },
  PPUSCROLL: {
    name: 'PPUSCROLL',
    kind: CompletionItemKind.Constant,
    detail: 'PPUSCROLL = $2005 (PPU Fine Scroll Offset)',
    documentation: `**PPUSCROLL ($2005)** (Write x2):
- 1st write: Horizontal scroll offset (X)
- 2nd write: Vertical scroll offset (Y)`,
  },
  PPU_ADDR: {
    name: 'PPU_ADDR',
    kind: CompletionItemKind.Constant,
    detail: 'PPU_ADDR = $2006 (PPU VRAM Address Port)',
    documentation: `**PPU_ADDR ($2006)** (Write x2):
- 1st write: High byte of VRAM address (\`>addr\`)
- 2nd write: Low byte of VRAM address (\`<addr\`)`,
  },
  PPU_DATA: {
    name: 'PPU_DATA',
    kind: CompletionItemKind.Constant,
    detail: 'PPU_DATA = $2007 (PPU VRAM Data Port)',
    documentation: `**PPU_DATA ($2007)** (Read/Write): Access VRAM byte at current PPU_ADDR. Increments VRAM address by 1 or 32 based on PPU_CTRL bit 2.`,
  },
  OAM_DMA: {
    name: 'OAM_DMA',
    kind: CompletionItemKind.Constant,
    detail: 'OAM_DMA = $4014 (OAM Fast DMA Transfer)',
    documentation: `**OAM_DMA ($4014)** (Write-only): Writing high byte of CPU memory address (\`$02\` for \`$0200\`) triggers immediate 513-cycle DMA transfer of 256 bytes into sprite OAM.`,
  },
  APU_STATUS: {
    name: 'APU_STATUS',
    kind: CompletionItemKind.Constant,
    detail: 'APU_STATUS = $4015 (APU Sound Channel Control/Status)',
    documentation: `**APU_STATUS ($4015)**:
- Write: Enable sound channels (Bit 0: Pulse 1, Bit 1: Pulse 2, Bit 2: Triangle, Bit 3: Noise, Bit 4: DMC)
- Read: Check channel status and interrupt flags`,
  },
  JOYPAD1: {
    name: 'JOYPAD1',
    kind: CompletionItemKind.Constant,
    detail: 'JOYPAD1 = $4016 (Controller 1 Input Port)',
    documentation: `**JOYPAD1 ($4016)**:
- Write: Strobe shift registers (Write 1 then 0)
- Read: Read 8 button states serially (A, B, Select, Start, Up, Down, Left, Right)`,
  },
  JOYPAD2: {
    name: 'JOYPAD2',
    kind: CompletionItemKind.Constant,
    detail: 'JOYPAD2 = $4017 (Controller 2 Input Port)',
    documentation: `**JOYPAD2 ($4017)**: Controller 2 serial read port.`,
  },
};
