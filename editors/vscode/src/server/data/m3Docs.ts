import { CompletionItemKind, InsertTextFormat } from 'vscode-languageserver/node';

export interface DocEntry {
  name: string;
  kind: CompletionItemKind;
  detail: string;
  documentation: string;
  snippet?: string;
  insertTextFormat?: InsertTextFormat;
}

export const M3_TYPES: Record<string, DocEntry> = {
  uint8: {
    name: 'uint8',
    kind: CompletionItemKind.TypeParameter,
    detail: 'type uint8 (1 byte, 8-bit unsigned integer)',
    documentation: `**uint8** is an unsigned 8-bit integer with a range of \`0\` to \`255\` (\`$00\` to \`$FF\`).
    
It is the native machine word size of the MOS 6502 processor and performs single-cycle/low-overhead arithmetic.`,
  },
  int8: {
    name: 'int8',
    kind: CompletionItemKind.TypeParameter,
    detail: 'type int8 (1 byte, 8-bit signed integer)',
    documentation: `**int8** is a signed 8-bit two's-complement integer with a range of \`-128\` to \`127\`.
    
Mapped directly to 6502 accumulator/memory operations with signed branch testing (\`BMI\`, \`BPL\`, \`BVS\`, \`BVC\`).`,
  },
  uint16: {
    name: 'uint16',
    kind: CompletionItemKind.TypeParameter,
    detail: 'type uint16 (2 bytes, 16-bit unsigned integer)',
    documentation: `**uint16** is an unsigned 16-bit little-endian integer with a range of \`0\` to \`65,535\` (\`$0000\` to \`$FFFF\`).
    
Stored as low byte first, high byte second. Commonly used for memory addresses, pointers, and game state counters.`,
  },
  int16: {
    name: 'int16',
    kind: CompletionItemKind.TypeParameter,
    detail: 'type int16 (2 bytes, 16-bit signed integer)',
    documentation: `**int16** is a signed 16-bit little-endian integer with a range of \`-32,768\` to \`32,767\`.
    
Handled via 16-bit 6502 math sequences using carry chaining.`,
  },
  uint32: {
    name: 'uint32',
    kind: CompletionItemKind.TypeParameter,
    detail: 'type uint32 (4 bytes, 32-bit unsigned integer)',
    documentation: `**uint32** is an unsigned 32-bit little-endian integer with a range of \`0\` to \`4,294,967,295\`.
    
Useful for high scores, cumulative game timers, and large fixed-point accumulators.`,
  },
  int32: {
    name: 'int32',
    kind: CompletionItemKind.TypeParameter,
    detail: 'type int32 (4 bytes, 32-bit signed integer)',
    documentation: `**int32** is a signed 32-bit little-endian integer with a range of \`-2,147,483,648\` to \`2,147,483,647\`.`,
  },
  bool: {
    name: 'bool',
    kind: CompletionItemKind.TypeParameter,
    detail: 'type bool (1 byte, boolean)',
    documentation: `**bool** represents a boolean flag occupying 1 byte:
- \`true\` is represented as \`1\`
- \`false\` is represented as \`0\``,
  },
  string: {
    name: 'string',
    kind: CompletionItemKind.TypeParameter,
    detail: 'type string (array of uint8 / byte sequence)',
    documentation: `**string** represents an immutable sequence of ASCII character bytes, typically stored in PRG-ROM or used in data tables.`,
  },
};

export const M3_STORAGE: Record<string, DocEntry> = {
  zp: {
    name: 'zp',
    kind: CompletionItemKind.Keyword,
    detail: 'storage zp (Zero Page RAM: $0000 - $00FF)',
    documentation: `**zp** (alias **zeropage**) allocates the variable in the CPU Zero Page memory region (\`$0000\`-\`$00FF\`, 256 bytes).

### Characteristics:
- **Fastest Addressing**: Instructions using Zero Page addressing are 1 byte smaller and 1 cycle faster.
- **Indirect Pointers**: Required for 16-bit pointers used in \`[ptr], Y\` indirect indexed addressing.
- **Ideal For**: Hot loop counters, frame parameters, temporary scratchpads, and active entity pointers.`,
  },
  zeropage: {
    name: 'zeropage',
    kind: CompletionItemKind.Keyword,
    detail: 'storage zeropage (alias of zp: $0000 - $00FF)',
    documentation: `**zeropage** is an alias for **zp**. Allocates variables in CPU Zero Page memory (\`$0000\`-\`$00FF\`).`,
  },
  ram: {
    name: 'ram',
    kind: CompletionItemKind.Keyword,
    detail: 'storage ram (Default CPU RAM: $0300 - $07FF)',
    documentation: `**ram** (alias **bss**) allocates variables in standard internal NES CPU RAM (\`$0300\`-\`$07FF\`, 1280 bytes).

### Memory Map:
- \`$0000 - $00FF\`: Zero Page (\`zp\`)
- \`$0100 - $01FF\`: Hardware CPU Stack
- \`$0200 - $02FF\`: OAM Sprite Buffer
- \`$0300 - $07FF\`: User RAM (\`ram\`) — *Default when no storage class is specified.*`,
  },
  wram: {
    name: 'wram',
    kind: CompletionItemKind.Keyword,
    detail: 'storage wram (MMC3 Work RAM / PRG-RAM: $6000 - $7FFF)',
    documentation: `**wram** (alias **workram**) allocates variables in the MMC3 8KB Work RAM segment at \`$6000\`-\`$7FFF\`.

### Characteristics:
- 8 KB of space for large buffers, tile maps, and level data.
- Can be battery-backed for persistent save data.
- Linker automatically flags battery-backed RAM in the iNES header when used.`,
  },
  workram: {
    name: 'workram',
    kind: CompletionItemKind.Keyword,
    detail: 'storage workram (alias of wram: $6000 - $7FFF)',
    documentation: `**workram** is an alias for **wram**. Allocates variables in MMC3 8KB Work RAM at \`$6000\`-\`$7FFF\`.`,
  },
  bank: {
    name: 'bank',
    kind: CompletionItemKind.Keyword,
    detail: 'bank <index | auto> (PRG-ROM Bank Placement)',
    documentation: `**bank** specifies the 8KB PRG-ROM bank where a function or constant data table resides.

### Usage:
\`\`\`go
func update_sprites() bank 1 { ... }
const title_palette uint8[16] bank 0 = [16]uint8{...}
func render_hud() bank auto { ... }
\`\`\`

- \`bank <0..63>\`: Assigns code/data to a fixed or switched 8KB bank.
- \`bank auto\`: Tells the \`m3\` linker to automatically pack the symbol into an optimal available PRG bank.`,
  },
  auto: {
    name: 'auto',
    kind: CompletionItemKind.Keyword,
    detail: 'bank auto (Link-Time Automatic Bank Packing)',
    documentation: `**auto** is used with \`bank auto\` to let the \`m3\` linker automatically fit code or data into available 8KB PRG-ROM banks without manual placement.`,
  },
};

export const M3_INTRINSICS: Record<string, DocEntry> = {
  low: {
    name: 'low',
    kind: CompletionItemKind.Function,
    detail: 'low(value: uint16) -> uint8',
    documentation: `**low(val)** extracts the low 8-bit byte of a 16-bit word or address (\`val & $FF\`).

Equivalent to the assembly low-byte selector \`<val\`.

### Example:
\`\`\`go
var lo = low(ptr_address)
\`\`\``,
  },
  high: {
    name: 'high',
    kind: CompletionItemKind.Function,
    detail: 'high(value: uint16) -> uint8',
    documentation: `**high(val)** extracts the high 8-bit byte of a 16-bit word or address (\`(val >> 8) & $FF\`).

Equivalent to the assembly high-byte selector \`>val\`.

### Example:
\`\`\`go
var hi = high(ptr_address)
\`\`\``,
  },
  bank: {
    name: 'bank',
    kind: CompletionItemKind.Function,
    detail: 'bank(symbol) -> uint8',
    documentation: `**bank(symbol)** returns the 8KB PRG-ROM bank number containing the specified symbol.

Equivalent to the assembly bank selector \`^symbol\`.

### Example:
\`\`\`go
var b = bank(level_data)
\`\`\``,
  },
};

export const M3_KEYWORDS: Record<string, DocEntry> = {
  var: {
    name: 'var',
    kind: CompletionItemKind.Keyword,
    detail: 'var <name> <type>[length] [storage]',
    documentation: `**var** declares one or more mutable variables in RAM (\`zp\`, \`ram\`, or \`wram\`).

### Syntax:
\`\`\`go
var player_x uint8 zp
var score uint32 ram
var enemies Enemy[16] ram
var (
    cursor_x uint8 zp
    cursor_y uint8 zp
)
\`\`\``,
    snippet: 'var ${1:name} ${2:uint8}${3: zp}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  const: {
    name: 'const',
    kind: CompletionItemKind.Keyword,
    detail: 'const <name> <type>[length] [bank <n>] = <value>',
    documentation: `**const** defines immutable constant data or lookup tables stored in PRG-ROM.

### Syntax:
\`\`\`go
const sine_table uint8[32] bank auto = [32]uint8{ ... }
const palette uint8[16] bank 0 = [4]uint8{$0F, $00, $10, $30}
\`\`\``,
    snippet: 'const ${1:name} ${2:uint8}[${3:16}] ${4:bank auto} = ${5:[16]uint8{\\}$};',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  define: {
    name: 'define',
    kind: CompletionItemKind.Keyword,
    detail: 'define <name> <expr>',
    documentation: `**define** creates compile-time constants (hardware addresses, bitmasks, configuration numbers) that do not occupy PRG-ROM storage.

Emitted as assembler constant definitions (\`.define\`).

### Syntax:
\`\`\`go
define PPU_CTRL $2000
define (
    SCREEN_WIDTH 256
    MAX_LIVES    3
)
\`\`\``,
    snippet: 'define ${1:NAME} ${2:0}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  type: {
    name: 'type',
    kind: CompletionItemKind.Keyword,
    detail: 'type <Name> struct { ... }',
    documentation: `**type** declares a new named type or struct.

In \`m3\`, arrays of structs (\`var actors Actor[16]\`) are compiled as **Striped Data Structures (Structure of Arrays / SoA)** for maximum 6502 indexed addressing performance without 16-bit multiplication overhead.

### Syntax:
\`\`\`go
type Actor struct {
    x      uint8
    y      uint8
    vx     int8
    vy     int8
    health uint8
}
\`\`\``,
    snippet: 'type ${1:Name} struct {\n\t${2:x} ${3:uint8}\n}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  struct: {
    name: 'struct',
    kind: CompletionItemKind.Keyword,
    detail: 'struct { <field> <type> ... }',
    documentation: `**struct** defines a composite record structure. When declared as an array in \`m3\`, fields are decomposed into parallel striped arrays in memory.`,
  },
  func: {
    name: 'func',
    kind: CompletionItemKind.Keyword,
    detail: 'func <name>(<params>) [<return_type>] [bank <n>] { ... }',
    documentation: `**func** declares a function or procedure.

### Calling Convention:
- **Fastcall**: 1st param in \`A\`, 2nd in \`X\`, 3rd in \`Y\`.
- **Return Values**: 8-bit result in \`A\`; 16-bit in \`A\` (lo) and \`X\` (hi).
- **Inter-Bank Calls**: Handled automatically via trampoline when calling functions across PRG banks.

### Syntax:
\`\`\`go
func move_player(dx int8, dy int8) bank 0 {
    player_x += dx
    player_y += dy
}
\`\`\``,
    snippet: 'func ${1:name}(${2:params}) ${3:bank auto} {\n\t${0}\n}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  import: {
    name: 'import',
    kind: CompletionItemKind.Keyword,
    detail: 'import "<path>"',
    documentation: `**import** imports exported symbols from another \`.m3\` file or the standard library (\`pkg/data/lib/\`).

### Syntax:
\`\`\`go
import "oam.m3"
import "./player.m3"
import (
    "oam.m3"
    "./sprites.m3"
)
\`\`\``,
    snippet: 'import "${1:file.m3}"',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  package: {
    name: 'package',
    kind: CompletionItemKind.Keyword,
    detail: 'package <name>',
    documentation: `**package** specifies the package namespace of the compilation unit (typically \`main\`).`,
    snippet: 'package ${1:main}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  if: {
    name: 'if',
    kind: CompletionItemKind.Keyword,
    detail: 'if <condition> { ... } [else { ... }]',
    documentation: `**if** evaluates a condition and executes the associated block if true.

### Syntax:
\`\`\`go
if player_x > 240 {
    player_x = 240
} else {
    player_x += vx
}
\`\`\``,
    snippet: 'if ${1:condition} {\n\t${0}\n}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  else: {
    name: 'else',
    kind: CompletionItemKind.Keyword,
    detail: 'else { ... } | else if <condition> { ... }',
    documentation: `**else** provides fallback execution branch for an \`if\` statement.`,
  },
  for: {
    name: 'for',
    kind: CompletionItemKind.Keyword,
    detail: 'for [<init>; <cond>; <post>] | for <cond> | for { ... }',
    documentation: `**for** unifies looping in \`m3\`:
1. **Counted loop**: \`for i := uint8(0); i < 8; i++ { ... }\`
2. **While loop**: \`for player_health > 0 { ... }\`
3. **Infinite loop**: \`for { ... }\``,
    snippet: 'for ${1:i := uint8(0)}; ${2:i < 8}; ${3:i++} {\n\t${0}\n}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  switch: {
    name: 'switch',
    kind: CompletionItemKind.Keyword,
    detail: 'switch <expr> { case <v>: ... default: ... }',
    documentation: `**switch** matches an expression against multiple discrete values.

### Syntax:
\`\`\`go
switch game_state {
case 0:
    init_title()
case 1:
    run_gameplay()
default:
    game_state = 0
}
\`\`\``,
    snippet: 'switch ${1:game_state} {\ncase ${2:0}:\n\t${0}\ndefault:\n\tbreak\n}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  case: {
    name: 'case',
    kind: CompletionItemKind.Keyword,
    detail: 'case <value>:',
    documentation: `**case** specifies a branch value in a \`switch\` statement.`,
  },
  default: {
    name: 'default',
    kind: CompletionItemKind.Keyword,
    detail: 'default:',
    documentation: `**default** specifies the fallback branch in a \`switch\` statement.`,
  },
  return: {
    name: 'return',
    kind: CompletionItemKind.Keyword,
    detail: 'return [<expr>]',
    documentation: `**return** returns execution from the current function, optionally returning an 8-bit or 16-bit value in the accumulator/X register.`,
  },
  break: {
    name: 'break',
    kind: CompletionItemKind.Keyword,
    detail: 'break',
    documentation: `**break** terminates the innermost \`for\` or \`switch\` statement.`,
  },
  continue: {
    name: 'continue',
    kind: CompletionItemKind.Keyword,
    detail: 'continue',
    documentation: `**continue** skips to the next iteration of the innermost \`for\` loop.`,
  },
  asm: {
    name: 'asm',
    kind: CompletionItemKind.Keyword,
    detail: 'asm { <6502 assembly> }',
    documentation: `**asm** allows embedding inline 6502 assembly instructions directly inside an \`m3\` function for cycle-critical operations.

### Example:
\`\`\`go
asm {
    LDA #$00
    STA $2006
    STA $2006
}
\`\`\``,
    snippet: 'asm {\n\t${0}\n}',
    insertTextFormat: InsertTextFormat.Snippet,
  },
  true: {
    name: 'true',
    kind: CompletionItemKind.Keyword,
    detail: 'true (bool: 1)',
    documentation: `**true** represents boolean true value (\`1\`).`,
  },
  false: {
    name: 'false',
    kind: CompletionItemKind.Keyword,
    detail: 'false (bool: 0)',
    documentation: `**false** represents boolean false value (\`0\`).`,
  },
};
