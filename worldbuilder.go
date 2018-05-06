package mud

// WorldBuilder handles map generation on top of the World, which is more a data store.
type WorldBuilder interface {
	StepInto(x1, y1, x2, y2 uint32)
	World() World
	GetUser(string) User
}

// CellRenderInfo holds the minimum info for rendering a plot of map in a terminal
type CellRenderInfo struct {
	FGColor byte
	BGColor byte
	Glyph   rune
}

// SSHInterfaceTools has miscellaneous helpers for
type SSHInterfaceTools interface {
	GetTerrainMap(uint32, uint32, uint32, uint32) [][]CellRenderInfo
}

type worldBuilder struct {
	world World
}

func (builder *worldBuilder) StepInto(x1, y1, x2, y2 uint32) {
}

func (builder *worldBuilder) World() World {
	return builder.world
}

func (builder *worldBuilder) GetUser(username string) User {
	return builder.world.GetUser(username)
}

func (builder *worldBuilder) GetTerrainMap(cx, cy, width, height uint32) [][]CellRenderInfo {
	terrainMap := make([][]CellRenderInfo, height)
	for i := range terrainMap {
		terrainMap[i] = make([]CellRenderInfo, width)
	}

	startx := cx - (width / uint32(2))
	starty := cy - (height / uint32(2))

	worldWidth, worldHeight := builder.world.GetDimensions()

	for xd := int64(0); xd < int64(width); xd++ {
		for yd := int64(0); yd < int64(height); yd++ {
			if (int64(startx)+xd) >= 0 && (int64(startx)+xd) < int64(worldWidth) && (int64(starty)+yd) >= 0 && (int64(starty)+yd) < int64(worldHeight) {
				xcoord, ycoord := uint32(int64(startx)+xd), uint32(int64(starty)+yd)
				terrainInfo := CellTypes[builder.world.GetCellInfo(xcoord, ycoord).TerrainType]

				renderGlyph := rune('·')
				if len(terrainInfo.Representations) > 0 {
					renderGlyph = terrainInfo.Representations[(xcoord^ycoord)%uint32(len(terrainInfo.Representations))]
				} else {
					terrainInfo.FGcolor = 232
					terrainInfo.BGcolor = 233
				}

				terrainMap[yd][xd] = CellRenderInfo{
					FGColor: terrainInfo.FGcolor,
					BGColor: terrainInfo.BGcolor,
					Glyph:   renderGlyph}
			}
		}
	}

	return terrainMap
}

// NewWorldBuilder creates a new WorldBuilder to surround the World
func NewWorldBuilder(world World) WorldBuilder {
	return &worldBuilder{world: world}
}
