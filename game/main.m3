package main

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
    memory.Copy(data.FontPal, &paletteMirror[0], 16)
    mmc.PopDataBank()
    mmc.PushDataBank(^data.SpritePal)
    memory.Copy(data.SpritePal, &paletteMirror[16], 16)
    mmc.PopDataBank()
    ppu.DirectUploadPalette(paletteMirror)
    mmc.PushDataBank(^data.FontChr)
    ppu.DirectUpload(data.FontChr, 0, 4096)
    mmc.PopDataBank()
    mmc.PushDataBank(^data.SpriteChr)
    ppu.DirectUpload(data.SpriteChr, 4096, 4096)
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
