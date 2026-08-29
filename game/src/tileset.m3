package tileset

// Tile represents a single tile definition in a tile set.
type Tile struct {
    Chr uint8[4]
    Palette uint8
    BlocksVis bool
    Walkable bool
    Sailable bool
}

// TileSet represents the metadata of a tile set. The actual tiles are stored in
// bulk data and copied to RAM.
type TileSet struct {
    TilesChrSlot uint8 // Which CHR file to load from data.TilesChr into PPU $0800.
    TilesPaletteSlot uint8 // Which palette to use from data.TilesPal.
    TilesSlot uint8 // Which tile set data to use from data.TilesData.
}

// TileSets is all of the tile sets supported by the game.
data TileSets TileSet[] bank 61 = {
    {
        TilesChrSlot: 1,
        TilesPaletteSlot: 0,
    }
} 
