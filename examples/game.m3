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
