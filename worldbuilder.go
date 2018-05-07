package mud

import "math/rand"

// WorldBuilder handles map generation on top of the World, which is more a data store.
type WorldBuilder interface {
	StepInto(x1, y1, x2, y2 uint32)
	World() World
	GetUser(string) User

	MoveUserNorth(user User)
	MoveUserSouth(user User)
	MoveUserEast(user User)
	MoveUserWest(user User)
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
	newCell := builder.world.GetCellInfo(x2, y2)

	if newCell == nil {
		currentCell := builder.world.GetCellInfo(x1, y1)

		if currentCell == nil {
			return
		}

		cellType := CellTypes[currentCell.TerrainType]

		newCellType := cellType.Transitions[rand.Uint64()%uint64(len(cellType.Transitions))]

		newCellItem, ok := CellTypes[newCellType]
		if !ok {
			newCellItem = CellTypes[DefaultCellType]
		}

		PopulateCellFromAlgorithm(x1, y1, x2, y2, builder.world, &newCellItem)
	}
}

func (builder *worldBuilder) World() World {
	return builder.world
}

func (builder *worldBuilder) GetUser(username string) User {
	return builder.world.GetUser(username)
}

func (builder *worldBuilder) MoveUserNorth(user User) {
	location := user.Location()

	ci := builder.world.GetCellInfo(location.X, location.Y)
	if (ci != nil) && (ci.ExitBlocks&NORTHBIT != 0) {
		user.Log("Can't move north from here.")
		return
	}

	if location.Y > 0 {
		builder.StepInto(location.X, location.Y, location.X, location.Y-1)

		newcell := builder.world.GetCellInfo(location.X, location.Y-1)
		if (newcell != nil) && (newcell.ExitBlocks&SOUTHBIT != 0) {
			user.Log("Something is blocking your northward passage.")
			return
		}
		user.MoveNorth()
	}
}

func (builder *worldBuilder) MoveUserSouth(user User) {
	location := user.Location()
	_, height := builder.world.GetDimensions()

	ci := builder.world.GetCellInfo(location.X, location.Y)
	if (ci != nil) && (ci.ExitBlocks&SOUTHBIT != 0) {
		user.Log("Can't move south from here.")
		return
	}

	if location.Y < height-1 {
		builder.StepInto(location.X, location.Y, location.X, location.Y+1)

		newcell := builder.world.GetCellInfo(location.X, location.Y+1)
		if (newcell != nil) && (newcell.ExitBlocks&NORTHBIT != 0) {
			user.Log("Something is blocking your southward passage.")
			return
		}
		user.MoveSouth()
	}
}

func (builder *worldBuilder) MoveUserEast(user User) {
	location := user.Location()

	ci := builder.world.GetCellInfo(location.X, location.Y)
	if (ci != nil) && (ci.ExitBlocks&EASTBIT != 0) {
		user.Log("Can't move east from here.")
		return
	}

	if location.X > 0 {
		builder.StepInto(location.X, location.Y, location.X+1, location.Y)

		newcell := builder.world.GetCellInfo(location.X+1, location.Y)
		if (newcell != nil) && (newcell.ExitBlocks&WESTBIT != 0) {
			user.Log("Something is blocking your westward passage.")
			return
		}
		user.MoveEast()
	}
}

func (builder *worldBuilder) MoveUserWest(user User) {
	location := user.Location()
	width, _ := builder.world.GetDimensions()

	ci := builder.world.GetCellInfo(location.X, location.Y)
	if (ci != nil) && (ci.ExitBlocks&WESTBIT != 0) {
		user.Log("Can't move west from here.")
		return
	}

	if location.X < width-1 {
		builder.StepInto(location.X, location.Y, location.X-1, location.Y)

		newcell := builder.world.GetCellInfo(location.X-1, location.Y)
		if (newcell != nil) && (newcell.ExitBlocks&EASTBIT != 0) {
			user.Log("Something is blocking your eastward passage.")
			return
		}
		user.MoveWest()
	}
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
				cellInfo := builder.world.GetCellInfo(xcoord, ycoord)

				terrainType := ""
				if cellInfo != nil {
					terrainType = cellInfo.TerrainType
				}

				terrainInfo := CellTypes[terrainType]

				renderGlyph := rune('·')
				if cellInfo != nil && len(terrainInfo.Representations) > 0 {
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
