package mud

import (
	"math/rand"
	"strconv"
)

type visitFunc func(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain)

var generationAlgorithms map[string]visitFunc

var defaultAlgorithm = "once"

func getIntSetting(settings map[string]string, settingName string, defaultValue int) int {
	if settings != nil {
		value, ok := settings[settingName]

		if ok && len(value) > 0 {
			val, err := strconv.Atoi(value)

			if err == nil {
				return val
			}
		}
	}

	return defaultValue
}

func getStringSetting(settings map[string]string, settingName string, defaultValue string) string {
	if settings != nil {
		value, ok := settings[settingName]

		if ok && len(value) > 0 {
			return value
		}
	}

	return defaultValue
}

func visitOnce(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	world.SetCellInfo(x2, y2, &CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
}

func tendril(x, y uint32, count uint64, world World, regionID uint64, cellTerrain *CellTerrain) {
	if count <= 0 {
		return
	}

	cell := world.GetCellInfo(x, y)
	if cell == nil {
		world.SetCellInfo(x, y, &CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
		count--
	} else if cell.TerrainID != cellTerrain.ID {
		count--

		// Can pass through this and keep on going
		if !cell.TerrainData.Permeable {
			return
		}
	}

	width, height := world.GetDimensions()
	if x > 1 && y > 1 && x < width-2 && y < height-2 {
		nx, ny := x, y
		num := rand.Int() % 4
		if num%2 == 0 {
			nx += uint32(num - 1)
		} else {
			ny += uint32(num - 2)
		}
		tendril(nx, ny, count, world, regionID, cellTerrain)
	}
}

func visitTendril(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	radius := getIntSetting(cellTerrain.AlgorithmParameters, "radius", 4)

	tendrilcount := getIntSetting(cellTerrain.AlgorithmParameters, "tendrilcount", radius)

	for i := 0; i < tendrilcount; i++ {
		tendril(x2, y2, uint64(radius), world, regionID, cellTerrain)
	}

	world.SetCellInfo(x2, y2, &CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})

	nx, ny := x2+(x2-x1), y2+(y2-y1)
	ci := world.GetCellInfo(nx, ny)
	if ci == nil {
		world.SetCellInfo(nx, ny, &CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
	}
}

func visitSpread(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	blocked := false

	world.SetCellInfo(x2, y2, &CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})

	xs, xe, ys, ye := -1, 1, -1, 1

	if x1 > x2 {
		xe = 0
	} else if x1 < x2 {
		xs = 0
	}

	if y1 > y2 {
		ye = 0
	} else if y1 < y2 {
		ys = 0
	}

	for xd := xs; xd <= xe; xd++ {
		for yd := ys; yd <= ye; yd++ {
			nx, ny := uint32(int(x2)+xd), uint32(int(y2)+yd)
			ci := world.GetCellInfo(nx, ny)
			if ci == nil {
				world.SetCellInfo(nx, ny, &CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
			} else {
				blocked = true
			}
		}
	}

	if blocked {
		visitTendril(x1, y1, x2, y2, world, regionID, cellTerrain)
	}
}

func visitPath(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	xd := int(x2) - int(x1)
	yd := int(y2) - int(y1)
	nx, ny := (int(x2)), (int(y2))
	neighborTerrain, ok := cellTerrain.AlgorithmParameters["neighbor"]
	endcap, endok := cellTerrain.AlgorithmParameters["endcap"]
	radius := getIntSetting(cellTerrain.AlgorithmParameters, "radius", 5)

	world.SetCellInfo(x1, y1,
		&CellInfo{
			TerrainID:    cellTerrain.ID,
			RegionNameID: regionID})

	if !ok {
		ci := world.GetCellInfo(uint32(int(x1)+(xd*-2)), uint32(int(y1)+(yd*-2)))
		if ci != nil {
			neighborTerrain = ci.TerrainID
		}
	}

	length := int(radius/2) + rand.Int()%int(radius/2)
	broken := false

	for i := 0; i < length; i++ {
		newCell := world.GetCellInfo(uint32(nx), uint32(ny))

		if newCell == nil || newCell.TerrainID == neighborTerrain {
			world.SetCellInfo(uint32(nx), uint32(ny),
				&CellInfo{
					TerrainID:    cellTerrain.ID,
					RegionNameID: regionID})

			neighborLeft := world.GetCellInfo(uint32(nx+yd), uint32(ny+xd))
			neightborRight := world.GetCellInfo(uint32(nx-yd), uint32(ny-xd))

			if neighborLeft == nil {
				world.SetCellInfo(uint32(nx+yd), uint32(ny+xd),
					&CellInfo{
						TerrainID:    neighborTerrain,
						RegionNameID: regionID})
			}
			if neightborRight == nil {
				world.SetCellInfo(uint32(nx-yd), uint32(ny-xd),
					&CellInfo{
						TerrainID:    neighborTerrain,
						RegionNameID: regionID})
			}
		} else {
			broken = true
			break
		}

		// Make trails jitter a little
		if rand.Int()%3 == 0 {
			if rand.Int()%2 == 0 {
				nx -= yd
				ny -= xd
			} else {
				nx += yd
				ny += xd
			}
		} else {
			nx += xd
			ny += yd
		}
	}

	if !broken && endok {
		newCell := world.GetCellInfo(uint32(nx), uint32(ny))

		if newCell == nil {
			world.SetCellInfo(uint32(nx), uint32(ny), &CellInfo{TerrainID: endcap, RegionNameID: regionID})

			if rand.Int()%3 > 0 {
				visitPath(uint32(nx), uint32(ny), uint32(nx+1), uint32(ny), world, regionID, cellTerrain)
				visitPath(uint32(nx), uint32(ny), uint32(nx-1), uint32(ny), world, regionID, cellTerrain)
				visitPath(uint32(nx), uint32(ny+1), uint32(nx), uint32(ny), world, regionID, cellTerrain)
				visitPath(uint32(nx), uint32(ny-1), uint32(nx), uint32(ny), world, regionID, cellTerrain)
			}
		}
	}
}

func getAvailableBox(x1, y1, x2, y2 uint32, world World, height, width int) (int, int, int, int, int, int, bool) {
	xd := int(x2) - int(x1)
	yd := int(y2) - int(y1)

	ux, lx, uy, ly := int(x2), int(x2), int(y2), int(y2)

	if yd == 0 {
		ly -= int(height / 2)
		uy += int(height / 2)

		if xd > 0 {
			ux += int(width)
		} else {
			lx -= int(width)
		}
	} else if xd == 0 {
		lx -= int(width / 2)
		ux += int(width / 2)

		if yd > 0 {
			uy += int(height)
		} else {
			ly -= int(height)
		}
	}

	free := true

BlockCheck:
	for xc := lx; xc <= ux; xc++ {
		for yc := ly; yc <= uy; yc++ {
			if world.GetCellInfo(uint32(xc), uint32(yc)) != nil {
				free = false
				break BlockCheck
			}
		}
	}

	return lx, ly, ux, uy, xd, yd, free
}

func visitDungeonRoom(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	minRadius := getIntSetting(cellTerrain.AlgorithmParameters, "minradius", 5)
	maxRadius := getIntSetting(cellTerrain.AlgorithmParameters, "maxradius", 5)
	wall := getStringSetting(cellTerrain.AlgorithmParameters, "wall", cellTerrain.ID)
	exit := getStringSetting(cellTerrain.AlgorithmParameters, "exit", cellTerrain.ID)
	fallback := getStringSetting(cellTerrain.AlgorithmParameters, "fallback", cellTerrain.ID)

	radius := minRadius
	if (maxRadius - minRadius) > 0 {
		radius += rand.Int() % (maxRadius - minRadius)
	}

	lx, ly, ux, uy, xd, yd, free := getAvailableBox(x1, y1, x2, y2, world, radius*2, radius*2)

	if !free {
		mnx, mny, mxx, mxy := lx, ly, ux, uy
		if xd == 0 {
			mnx--
			mxx++
		} else if yd == 0 {
			mny--
			mxy++
		}

		for x := mnx; x <= mxx; x++ {
			for y := mny; y <= mxy; y++ {
				if world.GetCellInfo(uint32(x), uint32(y)) == nil {
					world.SetCellInfo(uint32(x), uint32(y), &CellInfo{TerrainID: wall, RegionNameID: regionID})
				}
			}
		}

		world.SetCellInfo(x2, y2, &CellInfo{TerrainID: fallback, RegionNameID: regionID})
	} else {
		for xdd := lx; xdd <= ux; xdd++ {
			for ydd := ly; ydd <= uy; ydd++ {
				if uint32(xdd) == x2 && uint32(ydd) == y2 {
					world.SetCellInfo(x2, y2, &CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
				} else if xdd == ux || xdd == lx || ydd == uy || ydd == ly {
					world.SetCellInfo(uint32(xdd), uint32(ydd), &CellInfo{TerrainID: wall, RegionNameID: regionID})
				} else {
					world.SetCellInfo(uint32(xdd), uint32(ydd), &CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
				}
			}
		}

		for _, pt := range []Point{
			Point{X: uint32(lx + (ux-lx)/2), Y: uint32(uy)},
			Point{X: uint32(lx + (ux-lx)/2), Y: uint32(ly)},
			Point{X: uint32(lx), Y: uint32(ly + (uy-ly)/2)},
			Point{X: uint32(ux), Y: uint32(ly + (uy-ly)/2)}} {
			world.SetCellInfo(pt.X, pt.Y, &CellInfo{TerrainID: exit, RegionNameID: regionID})
		}
	}
}

func visitGreatWall(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	settings := cellTerrain.AlgorithmParameters

	radius := getIntSetting(settings, "radius", 50)
	seedExit := getStringSetting(settings, "seed-exit", "clearing-grass")

	lx, ly, ux, uy, _, _, free := getAvailableBox(x1, y1, x2, y2, world, radius*2, radius*2)

	if free {
		newRegionID := world.NewPlaceID()
		wallThickness := getIntSetting(settings, "wall-thickness", 3)
		seedEntry := getStringSetting(settings, "seed-entry", "gravel")
		wallTexture := getStringSetting(settings, "wall-texture", "castle-clearing-wall")
		wallTextureInfo := CellInfo{
			TerrainID:    wallTexture,
			RegionNameID: newRegionID}
		entryTextureInfo := CellInfo{
			TerrainID:    seedEntry,
			RegionNameID: newRegionID}

		// Outline
		for x := 0; x < (ux - lx); x++ {
			if rand.Int()%2 == 0 {
				world.SetCellInfo(uint32(lx+x), uint32(uy), &wallTextureInfo)
				world.SetCellInfo(uint32(ux-x), uint32(ly), &wallTextureInfo)
				world.SetCellInfo(uint32(ux-x), uint32(uy-wallThickness), &wallTextureInfo)
				world.SetCellInfo(uint32(lx+x), uint32(ly+wallThickness), &wallTextureInfo)
			} else if x > 1 && x < (ux-lx)-1 {
				world.SetCellInfo(uint32(ux-x), uint32(uy-wallThickness), &entryTextureInfo)
				world.SetCellInfo(uint32(lx+x), uint32(ly+wallThickness), &entryTextureInfo)
			}
		}
		for y := 0; y < (uy - ly); y++ {
			if rand.Int()%2 == 0 {
				world.SetCellInfo(uint32(lx), uint32(uy-y), &wallTextureInfo)
				world.SetCellInfo(uint32(ux), uint32(ly+y), &wallTextureInfo)
				world.SetCellInfo(uint32(lx+wallThickness), uint32(ly+y), &wallTextureInfo)
				world.SetCellInfo(uint32(ux-wallThickness), uint32(uy-y), &wallTextureInfo)
			} else if y > 1 && y < (uy-ly)-1 {
				world.SetCellInfo(uint32(lx+wallThickness), uint32(ly+y), &entryTextureInfo)
				world.SetCellInfo(uint32(ux-wallThickness), uint32(uy-y), &entryTextureInfo)
			}
		}
		// Thick wall part
		for thickness := 1; thickness < wallThickness; thickness++ {
			for x := lx + thickness; x <= ux-thickness; x++ {
				world.SetCellInfo(uint32(x), uint32(ly+thickness), &wallTextureInfo)
				world.SetCellInfo(uint32(x), uint32(uy-thickness), &wallTextureInfo)
			}
			for y := ly + thickness; y <= uy-thickness; y++ {
				world.SetCellInfo(uint32(lx+thickness), uint32(y), &wallTextureInfo)
				world.SetCellInfo(uint32(ux-thickness), uint32(y), &wallTextureInfo)
			}
		}

		walkwayCell := CellInfo{
			TerrainID:    seedExit,
			RegionNameID: newRegionID}
		for i := 0; i <= wallThickness; i++ {
			midx, midy := lx+(ux-lx)/2, ly+(uy-ly)/2
			if i >= wallThickness-1 {
				walkwayCell.TerrainID = seedEntry
			}

			world.SetCellInfo(uint32(midx), uint32(ly+i), &walkwayCell)
			world.SetCellInfo(uint32(midx), uint32(uy-i), &walkwayCell)
			world.SetCellInfo(uint32(lx+i), uint32(midy), &walkwayCell)
			world.SetCellInfo(uint32(ux-i), uint32(midy), &walkwayCell)
		}
	} else {
		var cellTerrain *CellTerrain

		cellInfo := world.GetCellInfo(x1, y1)
		if cellInfo != nil {
			cellTerrain = &cellInfo.TerrainData
		} else {
			*cellTerrain = CellTypes[seedExit]
		}
		visitSpread(x1, y1, x2, y2, world, regionID, &cellInfo.TerrainData)
	}
}

func visitCircle(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
}

// PopulateCellFromAlgorithm will run the specified algorithm to generate terrain
func PopulateCellFromAlgorithm(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	if cellTerrain == nil {
		return
	}

	algo, ok := generationAlgorithms[cellTerrain.Algorithm]

	if !ok {
		algo = generationAlgorithms["once"]
	}

	algo(x1, y1, x2, y2, world, regionID, cellTerrain)
}

func init() {
	generationAlgorithms = make(map[string]visitFunc)

	generationAlgorithms["once"] = visitOnce
	generationAlgorithms["tendril"] = visitTendril
	generationAlgorithms["spread"] = visitSpread
	generationAlgorithms["path"] = visitPath
	generationAlgorithms["dungeon-room"] = visitDungeonRoom
	generationAlgorithms["great-wall"] = visitGreatWall
	generationAlgorithms["circle"] = visitCircle
}
