# m3 Assembly Language Specification (`SYNTAX.md`)

This document defines the syntax and grammar for the `m3` 6502 assembler. The assembly syntax is based closely on **ca65** conventions, tailored for NES software development targeting the MMC3 memory manager.

---

## 1. Source Structure and Lexical Conventions

### 1.1 Line Structure
Source files are UTF-8 encoded text files. A source line consists of up to four logical components:

```
[label:] [operation [operands]] [; comment]
```

All components are optional. Blank lines and lines containing only whitespace or comments are ignored.

### 1.2 Comments
- Line comments begin with a semicolon (`;`) and continue to the end of the line.
- End-of-line comments can appear after any statement or on a line by themselves.

```assembly
; Full-line comment
LDA #$00        ; End-of-line comment
```

### 1.3 Identifiers and Symbols
- **Global Identifiers**: Must start with a letter (`a-z`, `A-Z`) or underscore (`_`), followed by any combination of letters, digits (`0-9`), and underscores (`_`). Identifiers are case-sensitive by default.
- **Local Identifiers**: Begin with `@` (e.g., `@loop`, `@exit`, `@skip`). Local labels are scoped to the most recent preceding global label. Defining a new global label resets the local label namespace.
- **Anonymous Labels**: Named simply `:`. Referenced relatively using `:+`, `:++` (forward references to the 1st, 2nd next `:` label) and `:-`, `:--` (backward references to the 1st, 2nd previous `:` label).

Examples:
```assembly
player_update:      ; Global label
    LDA player_flags
    BEQ @skip       ; Local label reference
    JSR move_player
@skip:              ; Local label definition
    RTS

wait_vblank:
:   BIT $2002       ; Anonymous label
    BPL :-          ; Branch to previous anonymous label
    RTS
```

### 1.4 Literals and Constants

#### Integers
- **Decimal**: `10`, `255`, `1024`
- **Hexadecimal**: `$10`, `$00FF`, `$ABCD` or `0x10`, `0x00FF`
- **Binary**: `%10101010`, `%00001111` or `0b10101010`

#### Characters and Strings
- **Character Literals**: Enclosed in single quotes (`'A'`, `'\n'`, `'\''`). Evaluates to the ASCII / configured character code (8-bit value).
- **String Literals**: Enclosed in double quotes (`"Hello, World!\0"`). Supported escape sequences: `\n`, `\r`, `\t`, `\0`, `\\`, `\"`, `\'`.

---

## 2. Expressions and Operators

Expressions can be used anywhere an immediate value, address, or directive argument is expected. Constant expressions are evaluated at assembly time; relocatable expressions are resolved during linking.

### 2.1 Operators (Precedence from highest to lowest)

| Precedence | Operator | Description | Associativity |
|---|---|---|---|
| 1 (Unary) | `+`, `-` | Unary positive, negation | Right |
| 1 (Unary) | `~` | Bitwise NOT (1's complement) | Right |
| 1 (Unary) | `!` | Logical NOT (0 -> 1, non-zero -> 0) | Right |
| 1 (Unary) | `<` | Low byte selector (`expr & $FF`) | Right |
| 1 (Unary) | `>` | High byte selector (`(expr >> 8) & $FF`) | Right |
| 1 (Unary) | `^` | Bank byte selector (`(expr >> 16) & $FF`) | Right |
| 2 | `*`, `/`, `%` | Multiplication, integer division, modulo | Left |
| 3 | `+`, `-` | Addition, subtraction | Left |
| 4 | `<<`, `>>` | Bitwise shift left, logical shift right | Left |
| 5 | `<`, `<=`, `>`, `>=` | Relational comparisons (evaluates to 0 or 1) | Left |
| 6 | `==` (or `=`), `!=` (or `<>`) | Equality and inequality | Left |
| 7 | `&` | Bitwise AND | Left |
| 8 | `^` | Bitwise XOR | Left |
| 9 | `\|` | Bitwise OR | Left |
| 10 | `&&` | Logical AND | Left |
| 11 | `\|\|` | Logical OR | Left |

Parentheses `(` and `)` can be used to override default operator precedence.

### 2.2 Byte Selectors
Byte selectors extract individual bytes from 16-bit or 24-bit symbols and expressions:
```assembly
LDA #<palette_data   ; Load low byte of address
STA $00
LDA #>palette_data   ; Load high byte of address
STA $01
LDA #^palette_data   ; Load bank index/byte of symbol
```

---

## 3. Instruction Set and Addressing Modes

`m3` supports all standard MOS 6502 instructions across all 11 addressing modes:

| Addressing Mode | Syntax | Example | Description |
|---|---|---|---|
| **Implied** | `<mnemonic>` | `NOP`, `RTS`, `CLC` | No operand |
| **Accumulator** | `<mnemonic> A` or `<mnemonic>` | `LSR A`, `ROR` | Targets accumulator directly |
| **Immediate** | `<mnemonic> #<expr>` | `LDA #$10`, `LDX #<label` | 8-bit immediate constant |
| **Zero Page** | `<mnemonic> <expr>` | `LDA $10`, `STA player_x` | 8-bit memory address ($0000–$00FF) |
| **Zero Page, X** | `<mnemonic> <expr>, X` | `LDA $10, X`, `STA array, X` | Zero page indexed with X |
| **Zero Page, Y** | `<mnemonic> <expr>, Y` | `LDX $10, Y`, `STX array, Y` | Zero page indexed with Y |
| **Absolute** | `<mnemonic> <expr>` | `LDA $1234`, `JSR init_ppu` | 16-bit memory address |
| **Absolute, X** | `<mnemonic> <expr>, X` | `LDA $1234, X`, `LDA table, X` | 16-bit address indexed with X |
| **Absolute, Y** | `<mnemonic> <expr>, Y` | `LDA $1234, Y`, `LDA table, Y` | 16-bit address indexed with Y |
| **Indirect** | `JMP (<expr>)` | `JMP ($1234)`, `JMP (ptr)` | 16-bit indirect jump |
| **Indexed Indirect (Indirect, X)** | `<mnemonic> (<expr>, X)` | `LDA ($20, X)` | Zero-page pointer indexed by X |
| **Indirect Indexed (Indirect), Y** | `<mnemonic> (<expr>), Y` | `LDA ($20), Y` | Zero-page pointer indexed by Y |
| **Relative** | `<mnemonic> <expr>` | `BEQ @exit`, `BNE $04` | Signed 8-bit branch displacement |

### 3.1 Addressing Mode Disambiguation
For instructions where Zero Page vs. Absolute address sizes cannot be determined automatically (e.g. forward references to labels):
- Prefixing an expression with `z:` or `a:` forces the size:
  - `z:<expr>`: Forces zero-page addressing (1-byte address).
  - `a:<expr>`: Forces absolute addressing (2-byte address).

```assembly
LDA z:entity_state      ; Forces zero-page addressing mode
LDA a:entity_state      ; Forces absolute addressing mode
```

---

## 4. Assembler Directives

Directives begin with a leading dot (`.`) and provide instructions to the assembler.

### 4.1 Bank Selection Directive (MMC3 8KB Granularity)

The `.bank` directive informs the assembler which 8KB bank the following symbols and code belong to:

```assembly
.bank <bank_index>
```

- `<bank_index>`: An integer expression specifying the 8KB PRG bank number (e.g., `0` to `63` depending on ROM size).
- All labels and data defined after a `.bank` directive will inherit that bank context.
- The bank index is accessible via the `^` byte operator on symbols defined in that bank.

Example:
```assembly
.bank 0
dialog_table:
    .asciiz "Welcome to the world of m3!"

.bank 1
music_track_0:
    .incbin "assets/track0.bin"

.bank 63 ; Fixed bank in MMC3 ($E000-$FFFF)
reset_handler:
    SEI
    CLD
    JMP main
```

---

### 4.2 Constant Definition and Assignment

Symbols can be assigned constant values using `=` or `.set` / `.equ`:

```assembly
PPU_CTRL  = $2000
PPU_MASK  = $2001
MAX_LIVES = 3

; Reassignable symbols
temp_val .set 10
temp_val .set temp_val + 5
```

---

### 4.3 Data Definition Directives

#### `.byte` / `.byt` / `.db`
Emits one or more 8-bit byte values or string literals.

```assembly
.byte $01, $02, $03, $FF
.byte "NES", 0
```

#### `.word` / `.addr` / `.dw`
Emits one or more 16-bit little-endian words (low byte first, followed by high byte).

```assembly
.word $8000, $A000, main, reset_handler
```

#### `.dword` / `.dd`
Emits one or more 32-bit little-endian double words.

```assembly
.dword $00010000
```

#### `.asciiz` / `.stringz`
Emits an ASCII string literal followed by a null terminator byte (`$00`).

```assembly
.asciiz "GAME OVER"    ; Equivalent to .byte "GAME OVER", $00
```

#### `.res` / `.reserve`
Reserves a specified number of bytes. Optionally fills them with a given byte value (default: `$00`).

```assembly
.res 16          ; Reserves 16 bytes filled with $00
.res 64, $FF     ; Reserves 64 bytes filled with $FF
```

---

### 4.4 Symbol Scoping and Linkage Directives

#### `.export` / `.global`
Exports symbols so they can be referenced by other source files during linking.

```assembly
.export init_player, player_x, player_y
```

#### `.import`
Declares symbols defined and exported in other source files.

```assembly
.import load_palette, ppu_sync
.importzp ptr_temp   ; Imports a symbol residing in zero-page
```

#### `.proc` / `.endproc`
Defines a scoped procedural block. Symbols defined within the block are local to the procedure unless explicitly exported.

```assembly
.proc init_audio
    LDA #$00
    STA $4015
    RTS
.endproc
```

#### `.scope` / `.endscope`
Defines a generic lexical scope for symbols.

```assembly
.scope math
add:
    CLC
    ADC #$01
    RTS
.endscope

; Accessible externally via qualified name:
JSR math::add
```

---

### 4.5 Conditional Assembly

Conditional blocks enable or disable assembly of code blocks based on constant expressions.

```assembly
.if <expr>
    ; Code assembled if <expr> != 0
.elseif <expr>
    ; Optional alternative branch
.else
    ; Optional fallback branch
.endif
```

#### Symbol Existence Checks
```assembly
.ifdef SYMBOL_NAME
    ; Assembled if SYMBOL_NAME is defined
.endif

.ifndef SYMBOL_NAME
    ; Assembled if SYMBOL_NAME is NOT defined
.endif
```

---

### 4.6 Macro Definition

Macros allow parameterized template expansion:

```assembly
.macro set_ppu_addr addr
    LDA #>addr
    STA $2006
    LDA #<addr
    STA $2006
.endmacro

; Invocation:
    set_ppu_addr $2000
```

---

### 4.7 File Inclusion

#### `.include`
Includes another source file inline as if its contents were written in-place.

```assembly
.include "constants.inc"
.include "ppu.inc"
```

#### `.incbin`
Includes raw binary data directly into the assembled output with optional byte offset and length.

```assembly
.incbin "graphics.chr"
.incbin "level_data.bin", 0, 1024   ; Offset 0, length 1024 bytes
```

#### `.incchr`
Converts a PNG image directly into standard NES 2BPP planar CHR tile data (16 bytes per 8x8 pixel tile in row-major order). Image width and height must be multiples of 8 pixels.

```assembly
.bank 1
font_chr:
    .incchr "assets/font.png"

sprite_chr:
    .incchr "assets/player.png"
```

#### `.incpal`
Extracts palette colors from a PNG image and converts them to NES hardware 2C02 palette index bytes (`$00`–`$3F`). An optional second argument specifies the number of palette color bytes to emit (defaults to `4` for a single NES sub-palette, or up to `16` for full palettes).

```assembly
.bank 0
bg_palette:
    .incpal "assets/title.png"       ; Emits 4 bytes ($00-$3F)

full_palette:
    .incpal "assets/level.png", 16   ; Emits 16 bytes
```

---

### 4.5 Memory Allocation Directives (`.zp`, `.ram`, `.wram`)

`m3` provides dedicated directives for allocating uninitialized variables across the NES RAM spaces:

- **`.zp` / `.zeropage`**: Allocates into CPU Zero Page (`$0000`–`$00FF`, 256 bytes). Instructions referencing symbols in `.zp` automatically select 6502 Zero Page addressing modes (2 bytes).
- **`.ram` / `.bss`**: Allocates into CPU Main RAM (`$0300`–`$07FF`, 1280 bytes), reserving `$0100`–`$01FF` for the CPU Stack and `$0200`–`$02FF` for the OAM Sprite Buffer.
- **`.wram` / `.prgram` / `.sram`**: Allocates into MMC3 Battery-Backed Work RAM (`$6000`–`$7FFF`, 8192 bytes / 8KB). When `.wram` is used, the linker automatically sets the iNES battery-backed RAM header flag.

#### Section-Switching Syntax
Switch context to a RAM segment, then reserve bytes using `.res`:

```assembly
.zp
player_x:      .res 1
player_y:      .res 1
ptr_source:    .res 2

.ram
level_buffer:  .res 256
enemy_table:   .res 64

.wram
save_game:     .res 128
high_scores:   .res 32

.bank 0
main:
    LDA player_x      ; Generates Zero Page addressing (A5 00)
    STA player_y      ; Generates Zero Page addressing (85 01)
    LDA level_buffer  ; Generates Absolute addressing (AD 00 03)
    STA save_game     ; Generates Absolute addressing (8D 00 60)
```

#### Inline Allocation Syntax
Directly allocate bytes using `.zp <size>`, `.ram <size>`, or `.wram <size>`:

```assembly
player_x:      .zp 1
player_y:      .zp 1
ptr_source:    .zp 2
level_buffer:  .ram 256
save_game:     .wram 128
```

---

## 5. Formal EBNF Grammar

```ebnf
SourceFile      = { Statement | EOL } ;

Statement       = [ LabelDef ] [ Instruction | Directive | Assignment ] [ Comment ] EOL ;

LabelDef        = GlobalLabel | LocalLabel | AnonLabelDef ;
GlobalLabel     = Identifier ":" ;
LocalLabel      = "@" Identifier ":" ;
AnonLabelDef    = ":" ;

Assignment      = Identifier ( "=" | ".set" | ".equ" ) Expression ;

Instruction     = Mnemonic [ Operand ] ;
Operand         = Accumulator
                | Immediate
                | IndirectX
                | IndirectY
                | Indirect
                | MemoryIndexedX
                | MemoryIndexedY
                | MemoryOrRelative ;

Accumulator     = "A" ;
Immediate       = "#" Expression ;
IndirectX       = "(" Expression "," ( "X" | "x" ) ")" ;
IndirectY       = "(" Expression ")" "," ( "Y" | "y" ) ;
Indirect        = "(" Expression ")" ;
MemoryIndexedX  = Expression "," ( "X" | "x" ) ;
MemoryIndexedY  = Expression "," ( "Y" | "y" ) ;
MemoryOrRelative= Expression ;

Directive       = BankDir
                | MemoryDir
                | DataDir
                | ReserveDir
                | ExportDir
                | ImportDir
                | ScopeDir
                | CondDir
                | MacroDir
                | IncludeDir ;

BankDir         = ".bank" Expression ;
MemoryDir       = ( ".zp" | ".zeropage" | ".ram" | ".bss" | ".wram" | ".prgram" | ".sram" ) [ Expression ] ;
DataDir         = ( ".byte" | ".byt" | ".db" ) ExpressionList
                | ( ".word" | ".addr" | ".dw" ) ExpressionList
                | ( ".dword" | ".dd" ) ExpressionList
                | ( ".asciiz" | ".stringz" ) StringList ;

ReserveDir      = ".res" Expression [ "," Expression ] ;
ExportDir       = ( ".export" | ".global" ) IdentList ;
ImportDir       = ( ".import" | ".importzp" ) IdentList ;
ScopeDir        = ".proc" Identifier | ".endproc"
                | ".scope" [ Identifier ] | ".endscope" ;
IncludeDir      = ".include" StringLiteral
                | ".incbin" StringLiteral [ "," Expression [ "," Expression ] ]
                | ".incchr" StringLiteral
                | ".incpal" StringLiteral [ "," Expression ] ;

ExpressionList  = Expression { "," Expression } ;
StringList      = StringLiteral { "," StringLiteral } ;
IdentList       = Identifier { "," Identifier } ;

Expression      = LogicalOrExpr ;
LogicalOrExpr   = LogicalAndExpr { "||" LogicalAndExpr } ;
LogicalAndExpr  = BitOrExpr { "&&" BitOrExpr } ;
BitOrExpr       = BitXorExpr { "|" BitXorExpr } ;
BitXorExpr      = BitAndExpr { "^" BitAndExpr } ;
BitAndExpr      = EqualityExpr { "&" EqualityExpr } ;
EqualityExpr    = RelationalExpr { ( "==" | "=" | "!=" | "<>" ) RelationalExpr } ;
RelationalExpr  = ShiftExpr { ( "<" | "<=" | ">" | ">=" ) ShiftExpr } ;
ShiftExpr       = AddExpr { ( "<<" | ">>" ) AddExpr } ;
AddExpr         = MulExpr { ( "+" | "-" ) MulExpr } ;
MulExpr         = UnaryExpr { ( "*" | "/" | "%" ) UnaryExpr } ;
UnaryExpr       = ( "+" | "-" | "~" | "!" | "<" | ">" | "^" | "z:" | "a:" ) UnaryExpr
                | PrimaryExpr ;
PrimaryExpr     = Number | StringLiteral | Identifier | LocalIdent | AnonRef | "(" Expression ")" ;

LocalIdent      = "@" Identifier ;
AnonRef         = ":" ( "+" | "++" | "+++" | "-" | "--" | "---" ) ;
```
