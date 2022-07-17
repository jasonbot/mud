package mud

import (
	"math/rand"
	"time"
)

// MoveUser moves a user in the world; allowing the environment to intercept user movements
// in case some other thing needs to happen (traps, blocking, etc)
type MoveUser interface {
	MoveUserNorth(user User)
	MoveUserSouth(user User)
	MoveUserEast(user User)
	MoveUserWest(user User)
}

// WorldBuilder handles map generation on top of the World, which is more a data store.
type WorldBuilder interface {
	StepInto(x1, y1, x2, y2 uint32) bool
	World() World
	GetUser(string) User
	Chat(LogItem)
	Attack(interface{}, interface{}, *Attack)

	MoveUser
}

// CellRenderInfo holds the minimum info for rendering a plot of map in a terminal
type CellRenderInfo struct {
	FGColor byte
	BGColor byte
	Bold    bool
	Glyph   rune
}

// SSHInterfaceTools has miscellaneous helpers for
type SSHInterfaceTools interface {
	GetTerrainMap(uint32, uint32, uint32, uint32) [][]CellRenderInfo
}

type worldBuilder struct {
	world World
}

func (builder *worldBuilder) populateAround(x, y uint32, xdelta, ydelta int) {
	wwidth, wheight := builder.world.GetDimensions()

	if x > 100 && x < wwidth-100 && y > 100 && y < wheight-100 {
		for i := 1; i < 25; i++ {
			xd := uint32(int(x) + (rand.Int()%i - (i / 2)))
			yd := uint32(int(y) + (rand.Int()%i - (i / 2)))

			if builder.world.Cell(xd, yd).CellInfo() != nil {
				type diff struct {
					x, y int
				}

				directions := []diff{diff{x: -1, y: 0}, diff{x: 1, y: 0}, diff{x: 0, y: -1}, diff{x: 0, y: 1}}
				movement := directions[rand.Int()%len(directions)]

				if builder.world.Cell(uint32(int(xd)+movement.x), uint32(int(yd)+movement.y)).CellInfo() == nil {
					builder.StepInto(xd, yd, uint32(int(xd)+xdelta), uint32(int(yd)+ydelta))
				}
			}
		}
	}
}

func (builder *worldBuilder) StepInto(x1, y1, x2, y2 uint32) bool {
	newCell := builder.world.Cell(x2, y2)
	returnVal := newCell == nil

	if newCell.IsEmpty() {
		currentCell := builder.world.Cell(x1, y1)

		returnVal = PopulateCellFromAlgorithm(currentCell, newCell, builder.world)
	}

	return returnVal
}

func (builder *worldBuilder) World() World {
	return builder.world
}

func (builder *worldBuilder) GetUser(username string) User {
	return builder.world.GetUser(username)
}

func (builder *worldBuilder) Chat(message LogItem) {
	builder.world.Chat(message)
}

func (builder *worldBuilder) Attack(source interface{}, target interface{}, attack *Attack) {
	builder.world.Attack(source, target, attack)
}

func (builder *worldBuilder) MoveUserNorth(user User) {
	location := user.Location()

	ci := builder.world.Cell(location.X, location.Y).CellInfo()
	if (ci != nil) && (ci.ExitBlocks&NORTHBIT != 0) {
		return
	}

	if location.Y > 0 {
		if builder.StepInto(location.X, location.Y, location.X, location.Y-1) {
			builder.world.Cell(location.X, location.Y-1).ClearCreatures()
		}

		newcell := builder.world.Cell(location.X, location.Y-1).CellInfo()

		ct := CellTypes[ci.TerrainID]
		if newcell != nil {
			ct = CellTypes[newcell.TerrainID]
		}

		if (newcell == nil) || (newcell.ExitBlocks&SOUTHBIT != 0 || ct.Blocking) {
			return
		}
		user.MoveNorth()
		builder.populateAround(location.X, location.Y, 0, -1)
	}
}

func (builder *worldBuilder) MoveUserSouth(user User) {
	location := user.Location()
	_, height := builder.world.GetDimensions()

	ci := builder.world.Cell(location.X, location.Y).CellInfo()
	if (ci != nil) && (ci.ExitBlocks&SOUTHBIT != 0) {
		return
	}

	if location.Y < height-1 {
		if builder.StepInto(location.X, location.Y, location.X, location.Y+1) {
			builder.world.Cell(location.X, location.Y+1).ClearCreatures()
		}

		newcell := builder.world.Cell(location.X, location.Y+1).CellInfo()

		ct := CellTypes[DefaultCellType]
		if newcell != nil {
			ct = CellTypes[newcell.TerrainID]
		}

		if (newcell == nil) || (newcell.ExitBlocks&NORTHBIT != 0 || ct.Blocking) {
			return
		}
		user.MoveSouth()
		builder.populateAround(location.X, location.Y, 0, 1)
	}
}

func (builder *worldBuilder) MoveUserEast(user User) {
	location := user.Location()

	ci := builder.world.Cell(location.X, location.Y).CellInfo()
	if (ci != nil) && (ci.ExitBlocks&EASTBIT != 0) {
		return
	}

	if location.X > 0 {
		if builder.StepInto(location.X, location.Y, location.X+1, location.Y) {
			builder.world.Cell(location.X+1, location.Y).ClearCreatures()
		}

		newcell := builder.world.Cell(location.X+1, location.Y).CellInfo()

		ct := CellTypes[DefaultCellType]
		if newcell != nil {
			ct = CellTypes[newcell.TerrainID]
		}

		if (newcell == nil) || (newcell.ExitBlocks&WESTBIT != 0 || ct.Blocking) {
			return
		}
		user.MoveEast()
		builder.populateAround(location.X, location.Y, 1, 0)
	}
}

func (builder *worldBuilder) MoveUserWest(user User) {
	location := user.Location()
	width, _ := builder.world.GetDimensions()

	ci := builder.world.Cell(location.X, location.Y).CellInfo()
	if (ci != nil) && (ci.ExitBlocks&WESTBIT != 0) {
		return
	}

	if location.X < width-1 {
		if builder.StepInto(location.X, location.Y, location.X-1, location.Y) {
			builder.world.Cell(location.X-1, location.Y).ClearCreatures()
		}

		newcell := builder.world.Cell(location.X-1, location.Y).CellInfo()

		ct := CellTypes[DefaultCellType]
		if newcell != nil {
			ct = CellTypes[newcell.TerrainID]
		}

		if (newcell == nil) || (newcell.ExitBlocks&EASTBIT != 0 || ct.Blocking) {
			return
		}
		user.MoveWest()
		builder.populateAround(location.X, location.Y, -1, 0)
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
				cellInfo := builder.world.Cell(xcoord, ycoord).CellInfo()

				if cellInfo != nil {
					terrainInfo := cellInfo.TerrainData

					renderGlyph := rune('·')
					if cellInfo != nil && len(terrainInfo.Representations) > 0 {
						index := int64(xcoord ^ ycoord)
						if terrainInfo.Animated {
							index += time.Now().Unix()
						}
						renderGlyph = terrainInfo.Representations[uint32(index)%uint32(len(terrainInfo.Representations))]
					} else {
						terrainInfo.FGcolor = 232
						terrainInfo.BGcolor = 233
					}

					if cellInfo.TerrainData.Blocking == false {
						hasItems := false
						if builder.world.Cell(uint32(int64(startx)+xd), uint32(int64(starty)+yd)).HasInventoryItems() {
							hasItems = true
							terrainInfo.FGcolor = 178
							renderGlyph = rune('≡')
							terrainInfo.Bold = true
						}

						if builder.world.Cell(uint32(int64(startx)+xd), uint32(int64(starty)+yd)).HasCreatures() {
							if hasItems {
								terrainInfo.FGcolor = 175
								renderGlyph = rune('≜')
							} else {
								terrainInfo.FGcolor = 172
								renderGlyph = rune('∆')
								terrainInfo.Bold = true
							}
						}
					}

					terrainMap[yd][xd] = CellRenderInfo{
						FGColor: terrainInfo.FGcolor,
						BGColor: terrainInfo.BGcolor,
						Bold:    terrainInfo.Bold,
						Glyph:   renderGlyph}
				}
			}
		}
	}

	for _, player := range builder.world.OnlineUsers() {
		location := player.Location()
		if location.X >= startx && location.X < startx+width && location.Y >= starty && location.Y < starty+height {
			ix := location.X - startx
			iy := location.Y - starty

			terrainMap[iy][ix].FGColor = 160
			switch terrainMap[iy][ix].Glyph {
			case rune('⁂'):
				continue
			case rune('⁑'):
				terrainMap[iy][ix].Glyph = rune('⁂')
			case rune('*'):
				terrainMap[iy][ix].Glyph = rune('⁑')
			default:
				terrainMap[iy][ix].Glyph = rune('*')
			}
		}
	}

	return terrainMap
}

// NewWorldBuilder creates a new WorldBuilder to surround the World
func NewWorldBuilder(world World) WorldBuilder {
	return &worldBuilder{world: world}
}
