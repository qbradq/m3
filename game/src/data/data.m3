package data

data (
    FontChr         uint8[] bank 0  = incchr("gfx/font.png")
    TilesSurfaceChr uint8[] bank 0  = incchr("gfx/tiles_surface.png")
    SpriteChr       uint8[] bank 1  = incchr("gfx/sprites.png")
    TilesSurfacePal uint8[] bank 61 = incpal("gfx/tiles_surface.pal")
    SpritePal       uint8[] bank 61 = incpal("gfx/sprites.pal")
)
