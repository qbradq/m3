# m3 High-Level Language Specification (`docs/language.md`)

This document defines the syntax, semantics, and usage guidelines for the high-level programming language portion of `m3`. 

The language adopts **Go-like syntax and ergonomics** while mapping directly to the architectural realities of the **MOS 6502 processor** and the **NES MMC3 (Mapper 4)** memory manager. Rather than implementing a full Go runtime—which would be too heavy for an 8-bit NES—`m3` leverages Go's clean grammar to express low-level, high-efficiency constructs such as **striped data structs (Structure of Arrays)**, **explicit memory storage classes**, and **zero-overhead calling conventions**.

---

## 1. Architectural Model & Memory Layout

The NES contains limited internal memory and relies on mapper hardware for bank switching. `m3` exposes explicit storage classes that map variables to specific hardware memory regions.

```
+------------------+ $FFFF  -------------------------+
| Fixed PRG-ROM    |                                 |
| Bank 63 ($E000)  |  Fixed Bank 63: Runtime & Vecs  |
+------------------+ $DFFF  (512KB Total, 8KB Banks) |
| Fixed PRG-ROM    |                                 |
| Bank 62 ($C000)  |  Fixed Bank 62: PRG-ROM         |
+------------------+ $BFFF                           |
| Switchable PRG   |  Code Swap Bank ($A000-$BFFF)   |
| R7 Window        |  - Functions & `const` data     |
+------------------+ $9FFF                           |
| Switchable PRG   |  Dedicated Data Bank ($8000)    |
| R6 Window        |  - Banked `data` assets         |
+------------------+ $7FFF  -------------------------+
| MMC3 Work RAM    | 8KB Battery-Backed RAM (`wram` / `workram`)
+------------------+ $5FFF
| Expansion / I/O  | APU, PPU, MMC3 Registers
+------------------+ $0800
| Internal RAM     | $0300-$07FF: User RAM (`ram` - Default)
|                  | $0200-$02FF: OAM Sprite Buffer
|                  | $0100-$01FF: CPU Hardware Stack
+------------------+ $00FF
| Zero Page        | $0000-$00FF: Fast Direct RAM (`zp` / `zeropage`)
+------------------+ $0000
```

### 1.1 Storage Types

`m3` supports explicit RAM storage specifiers and dedicated ROM banking targets:

| Storage Specifier | Aliases | Address Range | Description |
| :--- | :--- | :--- | :--- |
| `zp` | `zeropage` | `$0000 - $00FF` (256 B) | High-speed single-byte addressing. Ideal for hot loop counters, pointers, and parameter scratchpads. |
| `ram` | *(default)* | `$0300 - $07FF` (1.25 KB) | Standard internal NES CPU RAM. Default storage when none is specified. |
| `wram` | `workram` | `$6000 - $7FFF` (8 KB) | MMC3 PRG-RAM segment. Often battery-backed for save games or used for large working buffers. |
| `data` | - | `$8000 - $9FFF` (8 KB) | Banked ROM data assets (CHR tiles, palettes, binary maps). Swapped into the R6 window when accessed. |

---

## 2. Source Organization, Scoping, and Imports

### 2.1 Source Files and Compilation Units
`m3` source files use the `.m3` extension. Each source file represents a distinct compilation unit that compiles to an object file (`.o`) or assembly output (`.s`).

### 2.2 The `import` Keyword
The `import` statement allows a source file to import other `.m3` files:
- **Library Includes** (paths not starting with `./` or `../`): Searched in the standard library (`pkg/data/lib/`, e.g. `import "oam.m3"` imports `pkg/data/lib/oam.m3`).
- **Relative Paths** (starting with `./` or `../`): Resolved relative to the directory containing the importing source file (e.g. `import "./player.m3"`, `import "../common/constants.m3"`).

Importing a file pulls all of its **exported symbols** (types, constants, variables, functions) into the current compilation unit as imported symbols.

```go
// Library include from pkg/data/lib
import "oam.m3"

// Relative paths
import "./player.m3"
import "../common/constants.m3"

// Grouped imports
import (
    "oam.m3"
    "./types.m3"
    "../audio/driver.m3"
)
```

- Paths are enclosed in double quotes and use forward slashes (`/`).
- Imported symbols become directly accessible in the file's scope.
- During code generation, references to imported variables and functions generate external symbol references (`.import` in assembly).

### 2.3 Symbol Scoping and Visibility
Symbol visibility follows Go conventions:
- **Exported Symbols**: Identifiers with an **uppercase** first letter (e.g., `PlayerX`, `InitActors`, `EnemyCount`) are exported and made available to other files that `import` this file.
- **Internal / Private Symbols**: Identifiers with a **lowercase** first letter or an underscore (e.g., `frame_count`, `local_temp`, `_hidden`) are private to the compilation unit.

### 2.4 Assembly Symbol Mangling
To facilitate clean interoperability with assembly routines and avoid collisions with 6502 instructions or hardware registers:
- All identifiers are prepended with an underscore (`_`) when emitted to assembly.
- For example, `Main` becomes `_Main`, `player_x` becomes `_player_x`, and `InitActors` becomes `_InitActors`.

### 2.5 Identifiers
- Identifiers must start with a letter (`a-z`, `A-Z`) or an underscore (`_`), followed by any combination of letters, digits (`0-9`), and underscores.
- Identifiers are case-sensitive.

### 2.6 Comments
`m3` supports both line comments and block comments:
```go
// This is a single-line comment

/*
   This is a multi-line
   block comment.
*/
```

### 2.7 Literals
- **Decimal**: `0`, `42`, `255`, `65535`
- **Hexadecimal**: `$FF`, `$8000` or `0xFF`, `0x8000`
- **Binary**: `%11001010` or `0b11001010`
- **Characters**: `'A'`, `'\n'`, `'\0'`
- **Strings**: `"Hello, World!"`, `"Level 1\0"`

---

## 3. Data Types

`m3` provides fixed-width integer types and basic aliases suited for 8-bit and 16-bit 6502 math.

| Type | Size | Range | Description |
| :--- | :--- | :--- | :--- |
| `uint8` | 1 byte (8-bit) | `0` to `255` | Unsigned 8-bit integer (native machine word). |
| `int8` | 1 byte (8-bit) | `-128` to `127` | Signed 8-bit two's-complement integer. |
| `uint16` | 2 bytes (16-bit) | `0` to `65,535` | Unsigned 16-bit little-endian integer. |
| `int16` | 2 bytes (16-bit) | `-32,768` to `32,767` | Signed 16-bit little-endian integer. |
| `uint32` | 4 bytes (32-bit) | `0` to `4,294,967,295` | Unsigned 32-bit little-endian integer (used for large scores/timers). |
| `int32` | 4 bytes (32-bit) | `-2,147,483,648` to `2,147,483,647` | Signed 32-bit little-endian integer. |
| `bool` | 1 byte (8-bit) | `true` (1), `false` (0) | Boolean flag. |
| `string` | Array of `uint8` | Byte sequence | Alias / array representation of `uint8`. |
| `*T` | 2 bytes (16-bit) | `$0000` to `$FFFF` | 16-bit pointer to type `T` (supports indirect addressing `(ptr),Y`). |

---

## 4. Variable Declarations (`var`)

Variables represent mutable data stored in Zero Page, internal RAM, or Work RAM.

### 4.1 Syntax

```go
var identifier type[length] storage
```

- **`length` (optional)**: The number of elements if declaring an array. If omitted, length is `1` (a scalar variable).
- **`storage` (optional)**: One of `zp`, `zeropage`, `ram`, `wram`, or `workram`. If omitted, defaults to `ram`.

### 4.2 Examples

```go
// Zero Page variables (fast access)
var player_x uint8 zp
var player_y uint8 zp
var ptr_screen *uint8 zp

// Internal RAM variables (default storage)
var player_score uint32          // defaults to ram
var enemy_states uint8[16] ram   // array of 16 bytes in RAM

// Work RAM variables (battery-backed or large buffers)
var save_data uint8[256] wram
var level_map uint8[1024] workram

// Grouped declarations
var (
    cursor_x  uint8 zp
    cursor_y  uint8 zp
    frame_cnt uint16 ram
)
```

---

## 5. Compile-Time Definitions (`define`), Constants (`const`), & Banked Data (`data`)

### 5.1 Compile-Time Definitions (`define`)

The `define` statement creates compile-time constant values (numeric constants, hardware addresses, bitmasks, arithmetic expressions) that do not occupy PRG-ROM storage. Instead, they are emitted directly as assembler definitions (`.define`).

#### Syntax
```go
// Single definition
define identifier const_expr

// Grouped definitions
define (
    identifier1 const_expr1
    identifier2 const_expr2
)
```

- **`const_expr`**: Any constant literal (integer, hex, binary, char), identifier, or constant arithmetic/bitwise expression. An optional `=` between identifier and expression is also accepted.
- Definitions starting with an uppercase letter are exported to assembly as `.export` symbols.

#### Examples
```go
// Hardware registers
define PPU_CTRL $2000
define PPU_MASK $2001
define PPU_STAT $2002

// Game constants and computed values
define (
    MAX_LIVES    3
    SCREEN_WIDTH 256
    HALF_WIDTH   (SCREEN_WIDTH / 2)
    BG_PALETTE   $3F00
)
```

---

### 5.2 Constant Definitions (`const`)

The `const` keyword defines immutable values or code-side lookup tables placed into **PRG-ROM** in the **Code Swap Bank (`$A000-$BFFF`)** (or fixed banks 62/63).

#### Syntax

```go
const identifier type[length] bank = value
```

- **`length` (optional)**:
  - If length `[n]` is specified and value has fewer elements, the rest is padded with zeroes (`0`).
  - If length `[n]` is specified and value has more elements, it is truncated or triggers a compile error.
  - If length is omitted (or `[]`), length is automatically inferred from the value initializer.
  - If length is `1` (or scalar), it is treated as a standard scalar constant placed in ROM.
- **`bank` (optional)**:
  - `bank <n>`: Places the constant into PRG-ROM bank `n` (`0` to `63`).
  - If bank is omitted, it defaults to **`bank auto`**, where the `m3` linker automatically packs the data into available PRG-ROM banks.
- **Address Relocation**: Relocated to the `$A000-$BFFF` code bank window.

#### Examples

```go
// Inferred length table placed in switchable code bank ($A000-$BFFF)
const sine_table uint8[] = [32]uint8{
    0, 24, 49, 73, 96, 117, 136, 153, 
    166, 177, 184, 187, 187, 182, 174, 162,
    147, 129, 109, 87, 63, 39, 15, 0,
    0, 0, 0, 0, 0, 0, 0, 0,
}

// Explicit fixed length table in Bank 0 (relocated to $A000)
const palette_data uint8[16] bank 0 = [4]uint8{$0F, $00, $10, $30} // Padded to 16 bytes

// String data in Fixed Bank 63 (relocated to $E000)
const title_string string[] bank 63 = "SUPER NES GAME\0"
```

---

### 5.3 Banked Data Storage (`data`)

The `data` keyword defines static assets (CHR graphics, palettes, binary level data, large asset arrays) placed into **PRG-ROM** that are relocated to the dedicated **Data Bank window (`$8000-$9FFF`)**.

When the HLL accesses `data` definitions, it switches the active PRG bank in the `$8000` window (MMC3 register 6).

#### Syntax

```go
// Single declaration
data identifier [bank n] = data_expr

// Grouped declaration
data (
    identifier1 [bank n1] = data_expr1
    identifier2 [bank n2] = data_expr2
)
```

- **`bank` (optional)**: Specifies explicit PRG-ROM bank (`0`–`63`) or `bank auto` (default).
- **`data_expr`**: An array literal (e.g. `[16]uint8{...}`), string literal, or data inclusion expression (`incbin(...)`, `incchr(...)`, `incpal(...)`).
- **Address Relocation**: All `data` symbols relocate to the `$8000-$9FFF` range.

#### Examples

```go
package assets

// Banked graphical and sound assets
data (
    TitleChr  bank 1 = incchr("title.png")
    TitlePal  bank 1 = incpal("title.png")
    WorldMap  bank 2 = incbin("world.bin")
    FontChr   = incchr("font.png")
)
```

---

## 6. Structs and Striped Data Structures (Structure of Arrays)

On the 6502, classic **Array of Structures (AoS)** (e.g., `struct { x, y, hp } actors[16]`) is computationally expensive because computing `actors[i].hp` requires multiplying `i` by the struct size or performing dynamic 16-bit pointer arithmetic.

`m3` solves this natively by implementing **Striped Data Structures (Structure of Arrays / SoA)**.

### 6.1 Struct Declaration

Struct types define composite records:

```go
type Actor struct {
    x       uint8
    y       uint8
    vx      int8
    vy      int8
    health  uint8
    flags   uint8
}
```

### 6.2 Striped Array Allocation

When an array of structs is declared, the compiler stripes the fields into parallel arrays:

```go
var actors Actor[16] ram
```

Under the hood, `m3` decomposes this declaration into separate, contiguous 16-byte arrays:
- `actors_x`: 16 bytes
- `actors_y`: 16 bytes
- `actors_vx`: 16 bytes
- `actors_vy`: 16 bytes
- `actors_health`: 16 bytes
- `actors_flags`: 16 bytes

### 6.3 Striped Member Access

You can access members using familiar Go syntax:

```go
// Accessing element fields using array-of-struct syntax
actors[i].x += actors[i].vx
actors[i].y += actors[i].vy

if actors[i].health == 0 {
    actors[i].flags = 0
}
```

This compiles to optimal single-instruction 6502 indexed addressing:
```assembly
LDX i
LDA actors_x, X
CLC
ADC actors_vx, X
STA actors_x, X
```

---

## 7. Functions & Procedures (`func`)

### 7.1 Syntax

```go
func identifier(param1 type, param2 type) return_type [bank n] {
    // Body
}
```

- **Parameters & Returns**: Parameter lists take the form `identifier type, ...`. Types must be explicit. Variadic arguments (`...`) are not supported.
- **`bank` Specifier**: Specifies which PRG-ROM bank contains this function. If omitted, defaults to `bank auto`.
- **Non-Reentrancy**: Functions are strictly **non-reentrant**. A function calling itself directly or indirectly is a compile-time error.
- **Memory Allocation**: Function parameters and local variables are allocated statically in RAM / Zero Page scratchpad rather than on a dynamic call stack.

### 7.2 Calling Conventions & Register Fastcall

To avoid the overhead of software stack frames, `m3` uses a high-performance calling convention:

1. **Register Fastcall**:
   - The first 8-bit parameter is passed in the **`A`** accumulator.
   - The second 8-bit parameter is passed in the **`X`** register.
   - The third 8-bit parameter is passed in the **`Y`** register.
2. **Static Zero Page Scratchpad Overlay**:
   - Additional parameters or 16-bit/32-bit values are passed via compiler-managed Zero Page scratchpad locations (`__arg0`, `__arg1`, ...).
   - Functions that do not call each other share overlapping parameter and local variable scratchpad space.
3. **Return Values**:
   - 8-bit results return in **`A`**.
   - 16-bit results return in **`A`** (low byte) and **`X`** (high byte).

### 7.3 Inter-Bank Far Calls

When calling a function located in a different PRG bank:
- The compiler automatically emits an MMC3 bank switch trampoline.
- The previous bank is preserved and restored upon return.

### 7.4 Interrupt Handlers

Special hardware entry points (NMI, IRQ, Reset) are annotated or exported:

```go
//export nmi
func handle_nmi() bank 63 {
    // VBlank handler (placed in fixed bank 63)
    ppu_transfer_oam()
    frame_counter++
}

//export reset
func handle_reset() bank 63 {
    // System boot logic
    init_hardware()
    main()
}
```

---

## 8. Control Flow

`m3` supports standard Go structured control flow statements.

### 8.1 If / Else

```go
if player_x > 240 {
    player_x = 240
} else if player_x < 8 {
    player_x = 8
} else {
    player_x += vx
}
```

### 8.2 For Loops

`m3` unifies looping through the `for` keyword:

```go
// 1. 8-bit Counted Loop
for i := uint8(0); i < 16; i++ {
    actors[i].health = 100
}

// 2. While-style Loop
for player_health > 0 {
    update_game()
}

// 3. Infinite Main Loop
for {
    wait_for_vblank()
    render()
}
```

### 8.3 Switch Statements

```go
switch game_state {
case 0:
    run_title_screen()
case 1:
    run_gameplay()
case 2:
    run_game_over()
default:
    game_state = 0
}
```

---

## 9. Operators & Expressions

### 9.1 Constant Expressions vs Variable Expressions
- **Compile-Time Constant Expressions**: For constant expressions (evaluated entirely at compile time), `m3` supports all standard math operators: `+`, `-`, `*`, `/`, `%`, `&`, `|`, `^`, `<<`, `>>`, and parenthesized sub-expressions.
- **Runtime Variable Expressions**: For expressions involving runtime variables (translated into 6502 machine instructions), only operations native to the 6502 architecture are supported directly in expressions (such as addition `+`, subtraction `-`, bitwise operations `&`, `|`, `^`, bit clear `&^`, bit shifts `<<`, `>>`, and increment/decrement `++`, `--`).

### 9.2 Arithmetic Operators
| Operator | Description | 6502 Implementation Note |
| :--- | :--- | :--- |
| `+` | Addition | Emits `CLC; ADC` |
| `-` | Subtraction | Emits `SEC; SBC` |
| `++`, `--` | Increment / Decrement | Emits `INC` / `DEC` or `INX` / `DEX` |

*(Note: Multiplication and division between variables are not native 6502 instructions; use bit-shifts or dedicated library helper routines).*

### 9.3 Bitwise Operators
| Operator | Description | 6502 Instruction |
| :--- | :--- | :--- |
| `&` | Bitwise AND | `AND` |
| `\|` | Bitwise OR | `ORA` |
| `^` | Bitwise XOR / NOT | `EOR` |
| `&^` | Bit Clear (AND NOT) | `EOR #$FF; AND` |
| `<<` | Shift Left | `ASL` |
| `>>` | Shift Right | `LSR` (unsigned) or `ROR` / arithmetic |

### 9.4 Byte and Address Extraction Built-ins
For interfacing with 16-bit addresses and banked assets:

- `low(val)` or `<val`: Extracts the low byte (`val & $FF`).
- `high(val)` or `>val`: Extracts the high byte (`(val >> 8) & $FF`).
- `bank(symbol)` or `^symbol`: Returns the 8KB PRG-ROM bank number of `symbol`.

### 9.5 Data Inclusion Expressions
`m3` provides built-in data inclusion expressions that import raw files, convert graphics to NES CHR tiles, or extract hardware palettes at compile time. Inclusion expressions can be used in place of any `uint8[]` literal value.

| Expression | Return Type | Description |
| :--- | :--- | :--- |
| `incbin(rel_path)` | `uint8[]` | Includes raw binary bytes directly from the specified file. |
| `incchr(rel_path)` | `uint8[]` | Converts a PNG image into standard NES 2BPP planar CHR tile data (16 bytes per 8x8 tile). Image width and height must be multiples of 8. |
| `incpal(rel_path [, n])` | `uint8[]` | Extracts palette colors from a PNG image and converts them to NES hardware 2C02 palette index bytes (`$00`–`$3F`). Optional count `n` defaults to `4` (single sub-palette) or up to `16`. |

#### File Path Resolution
All `rel_path` strings are resolved relative to the directory containing the source file.

#### Examples
```go
package data

// Include binary data
const raw_level uint8[] = incbin("levels/level1.bin")

// Convert PNG to CHR tile data
const font_chr uint8[] bank 1 = incchr("font.png")
const sprite_chr uint8[] bank 1 = incchr("sprites.png")

// Extract NES 2C02 palettes from PNG images
const bg_palette uint8[4] = incpal("title.png")
const font_pal uint8[16] = incpal("font.png", 16)

// Initialize RAM buffer with palette data
var fontPal uint8[16] = incpal("font.png", 16)
```

---

## 10. Inline Assembly Integration (`asm`)

For cycle-critical routines (e.g., PPU updates, split-screen raster IRQs), `m3` allows direct embedding of 6502 assembly within high-level functions.

```go
func ppu_set_address(addr uint16) {
    asm {
        LDA >addr       ; High byte
        STA $2006
        LDA <addr       ; Low byte
        STA $2006
    }
}

func wait_vblank() {
    asm {
    :   BIT $2002
        BPL :-
    }
}
```

- High-level variables and parameters can be referenced directly within `asm` blocks.
- Compiler labels and ca65-style anonymous labels (`:`, `:-`, `:+`) are supported.

---

## 11. Complete Example Program

The following example demonstrates a complete `m3` program with Zero Page variables, banked constant data, a striped actor table, and game logic:

```go
package main

// Hardware Registers
const (
    PPU_CTRL uint16 = $2000
    PPU_MASK uint16 = $2001
    PPU_STAT uint16 = $2002
)

// Striped Struct Definition
type Enemy struct {
    x      uint8
    y      uint8
    hp     uint8
    active bool
}

// Memory Allocations
var (
    frame_counter uint16   zp
    player_x      uint8    zp
    player_y      uint8    zp
    enemies       Enemy[8] ram
    high_score    uint32   wram
)

// PRG-ROM Data Table (Auto placed by Linker)
const enemy_spawn_x uint8[8] = [8]uint8{16, 48, 80, 112, 144, 176, 208, 240}

// Initialize Enemies
func init_enemies() {
    for i := uint8(0); i < 8; i++ {
        enemies[i].x = enemy_spawn_x[i]
        enemies[i].y = 32
        enemies[i].hp = 5
        enemies[i].active = true
    }
}

// Update Enemy Logic
func update_enemies() {
    for i := uint8(0); i < 8; i++ {
        if enemies[i].active {
            enemies[i].y++
            if enemies[i].y > 220 {
                enemies[i].y = 32
            }
        }
    }
}

// Main Game Entry Point
func main() bank 0 {
    player_x = 120
    player_y = 180
    init_enemies()

    for {
        // Wait for VBlank
        asm {
        :   BIT $2002
            BPL :-
        }

        frame_counter++
        update_enemies()
    }
}
```
