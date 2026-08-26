package main

import (
    "oam.m3"
    "ppu.m3"
    "memory.m3"

    "./data/data.m3"
)

// Hardware Registers
define (
    PPU_CTRL $2000
    PPU_MASK $2001
    PPU_STAT $2002
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
    palette_buffer uint8[32] ram
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
func main() {
    // Init variables
    player_x = 120
    player_y = 180
    init_enemies()

    // Load RAM
    memory.Copy(data.font_pal, &palette_buffer[0], 16)
    memory.Copy(data.sprite_pal, &palette_buffer[16], 16)

    // Initialize PPU
    ppu.Disable() 
    ppu.UploadPalette(palette_buffer)
    ppu.Enable()

    for {
        // Wait for VBlank
        asm {
        :   BIT $2002
            BPL :-
        }

        frame_counter++
        update_enemies()
        oam.Clear()
        oam.AdvanceFlicker()
        for i := uint8(0); i < 8; i++ {
            if enemies[i].active {
                oam.PutSprite(enemies[i].x, enemies[i].y, 0, 0)
            }
        }
    }
}
