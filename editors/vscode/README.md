# m3 Language & Assembler Support for VS Code

Official Visual Studio Code extension providing language support, syntax highlighting, and Language Server Protocol (LSP) integration for the **m3** programming language and 6502 assembly dialect targeting the NES (MMC3).

## Features

- **High-Level `.m3` Language Support**:
  - Full syntax highlighting for Go-like constructs, functions, striped structs, and memory allocations.
  - Storage specifiers (`zp`, `zeropage`, `ram`, `wram`, `workram`).
  - Compile-time definitions (`define`) and ROM constants (`const`).
  - Embedded inline assembly blocks (`asm { ... }`) with integrated 6502 syntax highlighting.
  - Byte selectors (`low()`, `high()`, `bank()`, `<`, `>`, `^`).
- **m3 6502 Assembler Support (`.s`, `.asm`, `.inc`)**:
  - Complete MOS 6502 instruction set and addressing modes.
  - Directives (`.bank`, `.zp`, `.ram`, `.wram`, `.byte`, `.word`, `.dword`, `.asciiz`, `.res`, `.export`, `.import`, `.proc`, `.scope`, `.if`, `.macro`, `.include`, `.incbin`, `.incchr`, `.incpal`, `.define`, `.set`, `.equ`).
  - Scoped and anonymous labels (`:`, `:+`, `:-`, `@local`).
  - Number formats (Hex `$FF` / `0xFF`, Binary `%1010` / `0b1010`, Decimal).
- **Language Server Architecture**:
  - Background language server communicating over LSP for document synchronization, ready for upcoming features (diagnostics, symbol navigation, completions, and hover documentation).

## Building and Packaging

1. Install dependencies:
   ```bash
   npm install
   ```

2. Build the extension bundle:
   ```bash
   npm run build
   ```

3. (Optional) Package into a `.vsix` file using `@vscode/vsce`:
   ```bash
   npx @vscode/vsce package
   ```
