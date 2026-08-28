package tileset

type Tile struct {
    chr uint8[4]
    palette uint8
    walkable bool
    sailable bool
}

var TileSet Tile[64] wram
