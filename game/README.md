# Maxima

Maxima is an NES game in the style of the DOS releases of the early Ultima
series.

## Development

Maxima uses the m3 language, defined at ../docs/language.md.

**Build**

```bash
go run main.go build -o maxima.nes game/src/main.m3
```

**Run**

Maxima is compatible with most NES emulators, including Mesen and FCEUX. In
order to run Maxima on hardware, you need a programmable NES cartridge that can
meet these requirements:
- Mapper 4 / MMC3
- 512KB PRG-ROM
- 8KB WRAM
- 8KB CHR-RAM
