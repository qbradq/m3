# m3 High-Level Language Specification (`docs/language.md`)

This document defines the syntax, semantics, and usage guidelines for the high-level programming language portion of `m3`. 

The language adopts **Go-like syntax and ergonomics** while mapping directly to the architectural realities of the **MOS 6502 processor** and the **NES MMC3 (Mapper 4)** memory manager. Rather than implementing a full Go runtime—which would be too heavy for an 8-bit NES—`m3` leverages Go's clean grammar to express low-level, high-efficiency constructs such as **striped data structs (Structure of Arrays)**, **explicit memory storage classes**, and **zero-overhead calling conventions**.

---

## 1. Architectural Model & Memory Layout

The NES contains limited internal memory and relies on mapper hardware for bank switching. `m3` exposes explicit storage classes that map variables to specific hardware memory regions.

```
+------------------+ $FFFF  -------------------------+
| Fixed PRG-ROM    |                                 |
| Bank 63 ($E000)  |        PRG-ROM Banks            |
+------------------+ $DFFF  (512KB Total, 8KB Banks) |
| Fixed PRG-ROM    |                                 |
| Bank 62 ($C000)  |  - Code & Const Data            |
+------------------+ $BFFF  - Bank 0..63             |
| Switchable PRG   |  - Bank 'auto' placement        |
| Bank 1 ($A000)   |                                 |
+------------------+ $9FFF                           |
| Switchable PRG   |                                 |
| Bank 0 ($8000)   |                                 |
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

`m3` supports three RAM storage specifiers:

| Storage Specifier | Aliases | Address Range | Description |
| :--- | :--- | :--- | :--- |
| `zp` | `zeropage` | `$0000 - $00FF` (256 B) | High-speed single-byte addressing. Ideal for hot loop counters, pointers, and parameter scratchpads. |
| `ram` | *(default)* | `$0300 - $07FF` (1.25 KB) | Standard internal NES CPU RAM. Default storage when none is specified. |
| `wram` | `workram` | `$6000 - $7FFF` (8 KB) | MMC3 PRG-RAM segment. Often battery-backed for save games or used for large working buffers. |

---

## 2. Lexical Structure & Comments

### 2.1 Identifiers
- Identifiers must start with a letter (`a-z`, `A-Z`) or an underscore (`_`), followed by any combination of letters, digits (`0-9`), and underscores.
- Identifiers are case-sensitive.
- Identifiers beginning with a capital letter or declared with `export` are accessible across compilation units.

### 2.2 Comments
`m3` supports both line comments and block comments:
```go
// This is a single-line comment

/*
   This is a multi-line
   block comment.
*/
```

### 2.3 Literals
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

## 5. Constant & ROM Data Definitions (`const`)

The `const` keyword defines immutable values or static data tables placed into **PRG-ROM**.

### 5.1 Syntax

```go
const identifier type[length] bank = value
```

- **`length` (optional)**:
  - If length `[n]` is specified and value has fewer elements, the rest is padded with zeroes (`0`).
  - If length `[n]` is specified and value has more elements, it is truncated or triggers a compile error.
  - If length is omitted (or `[]`), length is automatically inferred from the value initializer.
  - If length is `1` (or scalar), it is treated as a standard scalar constant.
- **`bank` (optional)**:
  - `bank <n>`: Places the data into PRG-ROM bank `n` (`0` to `63`).
  - If bank is omitted, it defaults to **`bank auto`**, where the `m3` linker automatically packs the data into available PRG-ROM banks.

### 5.2 Examples

```go
// Scalar compile-time constants (inlined or placed in ROM)
const MAX_LIVES uint8 = 3
const SCREEN_WIDTH uint8 = 256
const PPU_CTRL_ADDR uint16 = $2000

// Inferred length table placed with link-time auto banking
const sine_table uint8[] = [32]uint8{
    0, 24, 49, 73, 96, 117, 136, 153, 
    166, 177, 184, 187, 187, 182, 174, 162,
    147, 129, 109, 87, 63, 39, 15, 0,
    0, 0, 0, 0, 0, 0, 0, 0,
}

// Explicit fixed length table with zero-padding
const palette_data uint8[16] bank 0 = [4]uint8{$0F, $00, $10, $30} // Padded to 16 bytes

// String data in ROM
const title_string string[] bank 63 = "SUPER NES GAME\0"
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

- **Parameters & Returns**: Types must be explicit. Multiple return values are supported (returned in registers / scratchpad).
- **`bank` Specifier**: Specifies which PRG-ROM bank contains this function. If omitted, defaults to `bank auto`.

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

### 9.1 Arithmetic Operators
| Operator | Description | 6502 Implementation Note |
| :--- | :--- | :--- |
| `+` | Addition | Emits `CLC; ADC` |
| `-` | Subtraction | Emits `SEC; SBC` |
| `*` | Multiplication | Unsigned shift-and-add loop or hardware helper |
| `/` | Division | Fast shift-based division routine |
| `%` | Modulo | Fast remainder routine |
| `++`, `--` | Increment / Decrement | Emits `INC` / `DEC` or `INX` / `DEX` |

### 9.2 Bitwise Operators
| Operator | Description | 6502 Instruction |
| :--- | :--- | :--- |
| `&` | Bitwise AND | `AND` |
| `\|` | Bitwise OR | `ORA` |
| `^` | Bitwise XOR / NOT | `EOR` |
| `&^` | Bit Clear (AND NOT) | `EOR #$FF; AND` |
| `<<` | Shift Left | `ASL` |
| `>>` | Shift Right | `LSR` (unsigned) or `ROR` / arithmetic |

### 9.3 Byte and Address Extraction Built-ins
For interfacing with 16-bit addresses and banked assets:

- `low(val)` or `<val`: Extracts the low byte (`val & $FF`).
- `high(val)` or `>val`: Extracts the high byte (`(val >> 8) & $FF`).
- `bank(symbol)` or `^symbol`: Returns the 8KB PRG-ROM bank number of `symbol`.

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
