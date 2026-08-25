# m3

`m3` is a single-binary compiler and tool suite targeting the Nintendo
Entertainment System (NES) with the MMC3 memory manager (mapper 4).

## Overview

`m3` compiles a hybrid language designed for NES development that allows mixing
raw 6502 assembly with a strictly-typed, procedural language with Go-like syntax
and ergonomics.

Like the `go` tool itself, `m3` provides the entire toolchain (compiler,
assembler, linker, etc.) in a unified command-line binary.

## Documentation

- [Language Specification](docs/language.md): Grammar, storage classes, data types, striped structs, calling conventions, and high-level syntax.
- [Assembler Specification](docs/assembler.md): 6502 assembly syntax, directives, macros, expressions, and ca65 compatibility.

### Key Features

- **Target Platform**: NES with MMC3 memory management chip (mapper 4).
  - 512KB of PRG-ROM
  - 8KB of CHR-RAM
  - 8KB of WRAM
- **Hybrid Syntax**: Blend high-level strictly typed procedural constructs with
  inline 6502 assembly.
- **Toolchain**: Single CLI binary providing the full compilation pipeline.

## License

This project is licensed under the [MIT License](LICENSE).

