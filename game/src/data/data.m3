package data

data TilesChr uint8[][] bank 0 = {
    incchr("gfx/font.png"),
    incchr("gfx/tiles_surface.png"),
}

data SpritesChr uint8[][] bank 1 = {
    incchr("gfx/sprites.png"),
}

data TilesPal uint8[][] bank 61 = {
    incpal("gfx/tiles_surface.pal"),
}

data SpritesPal uint8[][] bank 61 = {
    incpal("gfx/sprites.pal"),
}
