package main

bank 63

import (
    "memory.m3"
    "mmc.m3"
    "ppu.m3"
    "ppu_driver.m3"

    "./data/data.m3"
)

var paletteMirror uint8[32] ram

func main() {
    // Init PPU RAM
    ppu.Disable()
    mmc.PushDataBank(^data.TilesPal)
    memory.Copy(data.TilesPal[0], &paletteMirror[0], 16)
    memory.Copy(data.SpritesPal[0], &paletteMirror[16], 16)
    mmc.PopDataBank()
    ppu.DirectUploadPalette(paletteMirror)

    mmc.PushDataBank(^data.TilesChr)
    ppu.DirectUpload(data.TilesChr[0], $0000, $0800)
    ppu.DirectUpload(data.TilesChr[1], $0800, $0800)
    mmc.PopDataBank()

    mmc.PushDataBank(^data.SpritesChr)
    ppu.DirectUpload(data.SpritesChr[0], $1000, $0400)
    mmc.PopDataBank()
    ppu.Enable()

    // Init PPU driver
    ppu_driver.Clear()

    // Main loop
    for {
        ppu.WaitForVBlank()
    }
}

func nmi() {
    ppu_driver.Process()
}
