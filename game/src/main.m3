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
    mmc.PushDataBank(^data.FontPal)
    memory.Copy(data.FontPal, &paletteMirror[0], 4)
    memory.Copy(data.TilesSurfacePal, &paletteMirror[4], 12)
    memory.Copy(data.SpritePal, &paletteMirror[16], 16)
    ppu.DirectUploadPalette(paletteMirror)
    ppu.DirectUpload(data.FontChr, $0000, $0800)
    ppu.DirectUpload(data.TilesSurfaceChr, $0800, $0800)
    ppu.DirectUpload(data.SpriteChr, $1000, $0400)
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
