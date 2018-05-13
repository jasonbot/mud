package mud

import (
	"fmt"
	"log"
	"math/rand"
)

// WorldBuilder handles map generation on top of the World, which is more a data store.
type WorldBuilder interface {
	StepInto(x1, y1, x2, y2 uint32) bool
	World() World
	GetUser(string) User
	Chat(LogItem)
	Attack(interface{}, *Attack)

	MoveUserNorth(user User)
	MoveUserSouth(user User)
	MoveUserEast(user User)
	MoveUserWest(user User)
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

			if builder.world.GetCellInfo(xd, yd) != nil {
				type diff struct {
					x, y int
				}

				directions := []diff{diff{x: -1, y: 0}, diff{x: 1, y: 0}, diff{x: 0, y: -1}, diff{x: 0, y: 1}}
				movement := directions[rand.Int()%len(directions)]

				if builder.world.GetCellInfo(uint32(int(xd)+movement.x), uint32(int(yd)+movement.y)) == nil {
					builder.StepInto(xd, yd, uint32(int(xd)+xdelta), uint32(int(yd)+ydelta))
				}
			}
		}
	}
}

func (builder *worldBuilder) StepInto(x1, y1, x2, y2 uint32) bool {
	newCell := builder.world.GetCellInfo(x2, y2)
	returnVal := newCell == nil

	if newCell == nil {
		currentCell := builder.world.GetCellInfo(x1, y1)

		if currentCell == nil {
			return returnVal
		}

		cellType := CellTypes[currentCell.TerrainID]

		newCellType := cellType.GetRandomTransition()

		if len(newCellType) == 0 {
			return false
		}

		if newCellType == "!previous" {
			newCellType = currentCell.TerrainID
		}

		newCellItem, ok := CellTypes[newCellType]
		if !ok {
			log.Printf("Found an invaid terrain type: %s", newCellType)
			newCellItem = CellTypes[DefaultCellType]
		}

		var regionID uint64
		if currentCell != nil {
			regionID = currentCell.RegionNameID
		} else {
			regionID = builder.World().NewPlaceID()
		}

		PopulateCellFromAlgorithm(x1, y1, x2, y2, builder.world, regionID, &newCellItem)
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

func (builder *worldBuilder) Attack(target interface{}, attack *Attack) {
	if attack == nil {
		log.Println("Attack is nil")
		return
	}
	var tap, trp, tmp uint64
	message := ""
	hitTarget := "target"

	user, userok := target.(User)
	creature, creatureok := target.(*Creature)

	var location *Point

	if userok {
		tap, trp, tmp = user.MaxAP(), user.MaxRP(), user.MaxMP()
		location = user.Location()
		hitTarget = user.Username()
	} else if creatureok {
		tap, trp, tmp = creature.CreatureTypeStruct.MaxAP, creature.CreatureTypeStruct.MaxRP, creature.CreatureTypeStruct.MaxMP
		location = &Point{X: creature.X, Y: creature.Y}
		hitTarget = creature.CreatureTypeStruct.Name
	}

	aap, arp, amp := attack.AP, attack.RP, attack.MP
	if trp > aap {
		aap = 0
	} else {
		aap -= trp
	}

	if tmp > arp {
		arp = 0
	} else {
		arp -= tmp
	}

	if tap > amp {
		amp = 0
	} else {
		amp -= tap
	}

	hit := rand.Int()%100 < int(attack.Accuracy)
	killed := false

	if hit {
		damage := aap + arp + amp

		if userok {
			if user.HP() > damage {
				user.SetHP(0)
				killed = true
			} else {
				user.SetHP(user.HP() - damage)
			}

			user.Save()
		} else if creatureok {
			if creature.HP > damage {
				creature.HP -= damage
			} else {
				creature.HP = 0
				killed = true
			}

			builder.world.UpdateCreature(creature)
		} else {
			log.Printf("How do I handle %v for attacks?", target)
		}

		if killed {
			message = fmt.Sprintf("%v took fatal damage from %v!", hitTarget, attack.Name)
		} else {
			message = fmt.Sprintf("%v hit %v for %v damage!", attack.Name, hitTarget, damage)
		}
	} else {
		message = fmt.Sprintf("%v missed!", attack.Name)
	}

	if len(message) > 0 {
		builder.Chat(LogItem{Message: message, MessageType: MESSAGEACTIVITY, Location: location})
	}
}

func (builder *worldBuilder) MoveUserNorth(user User) {
	location := user.Location()

	ci := builder.world.GetCellInfo(location.X, location.Y)
	if (ci != nil) && (ci.ExitBlocks&NORTHBIT != 0) {
		return
	}

	if location.Y > 0 {
		if builder.StepInto(location.X, location.Y, location.X, location.Y-1) {
			builder.world.ClearCreatures(location.X, location.Y-1)
		}

		newcell := builder.world.GetCellInfo(location.X, location.Y-1)

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

	ci := builder.world.GetCellInfo(location.X, location.Y)
	if (ci != nil) && (ci.ExitBlocks&SOUTHBIT != 0) {
		return
	}

	if location.Y < height-1 {
		if builder.StepInto(location.X, location.Y, location.X, location.Y+1) {
			builder.world.ClearCreatures(location.X, location.Y+1)
		}

		newcell := builder.world.GetCellInfo(location.X, location.Y+1)

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

	ci := builder.world.GetCellInfo(location.X, location.Y)
	if (ci != nil) && (ci.ExitBlocks&EASTBIT != 0) {
		return
	}

	if location.X > 0 {
		if builder.StepInto(location.X, location.Y, location.X+1, location.Y) {
			builder.world.ClearCreatures(location.X+1, location.Y)
		}

		newcell := builder.world.GetCellInfo(location.X+1, location.Y)

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

	ci := builder.world.GetCellInfo(location.X, location.Y)
	if (ci != nil) && (ci.ExitBlocks&WESTBIT != 0) {
		return
	}

	if location.X < width-1 {
		if builder.StepInto(location.X, location.Y, location.X-1, location.Y) {
			builder.world.ClearCreatures(location.X-1, location.Y)
		}

		newcell := builder.world.GetCellInfo(location.X-1, location.Y)

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
				cellInfo := builder.world.GetCellInfo(xcoord, ycoord)

				terrainType := ""
				if cellInfo != nil {
					terrainType = cellInfo.TerrainID
				}

				terrainInfo := CellTypes[terrainType]

				renderGlyph := rune('·')
				if cellInfo != nil && len(terrainInfo.Representations) > 0 {
					renderGlyph = terrainInfo.Representations[(xcoord^ycoord)%uint32(len(terrainInfo.Representations))]
				} else {
					terrainInfo.FGcolor = 232
					terrainInfo.BGcolor = 233
				}

				if builder.world.HasCreatures(uint32(int64(startx)+xd), uint32(int64(starty)+yd)) {
					terrainInfo.FGcolor = 172
					renderGlyph = rune('∆')
					terrainInfo.Bold = true
				}

				terrainMap[yd][xd] = CellRenderInfo{
					FGColor: terrainInfo.FGcolor,
					BGColor: terrainInfo.BGcolor,
					Bold:    terrainInfo.Bold,
					Glyph:   renderGlyph}
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
