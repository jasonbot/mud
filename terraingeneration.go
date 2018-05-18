package mud

import (
	"math"
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
	cell := world.Cell(x2, y2)
	cell.SetCellInfo(&CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
}

func tendril(x, y uint32, count uint64, world World, regionID uint64, cellTerrain *CellTerrain) {
	if count <= 0 {
		return
	}

	cell := world.Cell(x, y)
	cellInfo := cell.CellInfo()
	if cellInfo == nil {
		cell.SetCellInfo(&CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
		count--
	} else if cellInfo.TerrainID != cellTerrain.ID {
		count--

		// Can pass through this and keep on going
		if !cellInfo.TerrainData.Permeable {
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

	newCell := world.Cell(x2, y2)
	newCell.SetCellInfo(&CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})

	nx, ny := x2+(x2-x1), y2+(y2-y1)
	tCell := world.Cell(nx, ny)
	ci := tCell.CellInfo()
	if ci == nil {
		tCell.SetCellInfo(&CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
	}
}

func visitSpread(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	blocked := false

	newCell := world.Cell(x2, y2)
	newCell.SetCellInfo(&CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})

	//radius := getIntSetting(cellTerrain.AlgorithmParameters, 1)

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
			nxCell := world.Cell(nx, ny)
			ci := nxCell.CellInfo()
			if ci == nil {
				nxCell.SetCellInfo(&CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
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

	firstCell := world.Cell(x1, y1)

	firstCell.SetCellInfo(&CellInfo{
		TerrainID:    cellTerrain.ID,
		RegionNameID: regionID})

	if !ok {
		neighborCell := world.Cell(uint32(int(x1)+(xd*-2)), uint32(int(y1)+(yd*-2)))
		ci := neighborCell.CellInfo()
		if ci != nil {
			neighborTerrain = ci.TerrainID
		}
	}

	length := int(radius/2) + rand.Int()%int(radius/2)
	broken := false

	for i := 0; i < length; i++ {
		newCell := world.Cell(uint32(nx), uint32(ny))
		newCellInfo := newCell.CellInfo()

		if newCellInfo == nil || newCellInfo.TerrainID == neighborTerrain {
			newCell.SetCellInfo(&CellInfo{
				TerrainID:    cellTerrain.ID,
				RegionNameID: regionID})

			neighborLeft := world.Cell(uint32(nx+yd), uint32(ny+xd))
			neightborRight := world.Cell(uint32(nx-yd), uint32(ny-xd))

			if neighborLeft.CellInfo() == nil {
				neighborLeft.SetCellInfo(&CellInfo{
					TerrainID:    neighborTerrain,
					RegionNameID: regionID})
			}
			if neightborRight.CellInfo() == nil {
				neightborRight.SetCellInfo(&CellInfo{
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
		newCell := world.Cell(uint32(nx), uint32(ny))

		if newCell.CellInfo() == nil {
			newCell.SetCellInfo(&CellInfo{TerrainID: endcap, RegionNameID: regionID})

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
			cell := world.Cell(uint32(xc), uint32(yc))
			if cell.CellInfo() != nil {
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

	fallbackCell := world.Cell(x2, y2)
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
				setCell := world.Cell(uint32(x), uint32(y))
				if setCell.CellInfo() == nil {
					setCell.SetCellInfo(&CellInfo{TerrainID: fallback, RegionNameID: regionID})
				}
			}
		}

		fallbackCell.SetCellInfo(&CellInfo{TerrainID: fallback, RegionNameID: regionID})
	} else {
		for xdd := lx; xdd <= ux; xdd++ {
			for ydd := ly; ydd <= uy; ydd++ {
				xdydCell := world.Cell(uint32(xdd), uint32(ydd))
				if uint32(xdd) == x2 && uint32(ydd) == y2 {
					fallbackCell.SetCellInfo(&CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
				} else if xdd == ux || xdd == lx || ydd == uy || ydd == ly {
					xdydCell.SetCellInfo(&CellInfo{TerrainID: wall, RegionNameID: regionID})
				} else {
					xdydCell.SetCellInfo(&CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
				}
			}
		}

		for _, pt := range []Point{
			Point{X: uint32(lx + (ux-lx)/2), Y: uint32(uy)},
			Point{X: uint32(lx + (ux-lx)/2), Y: uint32(ly)},
			Point{X: uint32(lx), Y: uint32(ly + (uy-ly)/2)},
			Point{X: uint32(ux), Y: uint32(ly + (uy-ly)/2)}} {
			pt := world.Cell(pt.X, pt.Y)
			pt.SetCellInfo(&CellInfo{TerrainID: exit, RegionNameID: regionID})
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
				world.Cell(uint32(lx+x), uint32(uy)).SetCellInfo(&wallTextureInfo)
				world.Cell(uint32(ux-x), uint32(ly)).SetCellInfo(&wallTextureInfo)
				world.Cell(uint32(ux-x), uint32(uy-wallThickness)).SetCellInfo(&wallTextureInfo)
				world.Cell(uint32(lx+x), uint32(ly+wallThickness)).SetCellInfo(&wallTextureInfo)
			} else if x > 1 && x < (ux-lx)-1 {
				world.Cell(uint32(ux-x), uint32(uy-wallThickness)).SetCellInfo(&entryTextureInfo)
				world.Cell(uint32(lx+x), uint32(ly+wallThickness)).SetCellInfo(&entryTextureInfo)
			}
		}
		for y := 0; y < (uy - ly); y++ {
			if rand.Int()%2 == 0 {
				world.Cell(uint32(lx), uint32(uy-y)).SetCellInfo(&wallTextureInfo)
				world.Cell(uint32(ux), uint32(ly+y)).SetCellInfo(&wallTextureInfo)
				world.Cell(uint32(lx+wallThickness), uint32(ly+y)).SetCellInfo(&wallTextureInfo)
				world.Cell(uint32(ux-wallThickness), uint32(uy-y)).SetCellInfo(&wallTextureInfo)
			} else if y > 1 && y < (uy-ly)-1 {
				world.Cell(uint32(lx+wallThickness), uint32(ly+y)).SetCellInfo(&entryTextureInfo)
				world.Cell(uint32(ux-wallThickness), uint32(uy-y)).SetCellInfo(&entryTextureInfo)
			}
		}
		// Thick wall part
		for thickness := 1; thickness < wallThickness; thickness++ {
			for x := lx + thickness; x <= ux-thickness; x++ {
				world.Cell(uint32(x), uint32(ly+thickness)).SetCellInfo(&wallTextureInfo)
				world.Cell(uint32(x), uint32(uy-thickness)).SetCellInfo(&wallTextureInfo)
			}
			for y := ly + thickness; y <= uy-thickness; y++ {
				world.Cell(uint32(lx+thickness), uint32(y)).SetCellInfo(&wallTextureInfo)
				world.Cell(uint32(ux-thickness), uint32(y)).SetCellInfo(&wallTextureInfo)
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

			world.Cell(uint32(midx), uint32(ly+i)).SetCellInfo(&walkwayCell)
			world.Cell(uint32(midx), uint32(uy-i)).SetCellInfo(&walkwayCell)
			world.Cell(uint32(lx+i), uint32(midy)).SetCellInfo(&walkwayCell)
			world.Cell(uint32(ux-i), uint32(midy)).SetCellInfo(&walkwayCell)
		}
	} else {
		var cellTerrain *CellTerrain

		cellInfo := world.Cell(x1, y1).CellInfo()
		if cellInfo != nil {
			cellTerrain = &cellInfo.TerrainData
		} else {
			*cellTerrain = CellTypes[seedExit]
		}
		visitSpread(x1, y1, x2, y2, world, regionID, &cellInfo.TerrainData)
	}
}

func visitCircle(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	settings := cellTerrain.AlgorithmParameters

	radius := getIntSetting(settings, "radius", 50)
	entryRadius := getIntSetting(settings, "entry-radius", radius)
	seedExit := getStringSetting(settings, "seed-exit", "clearing-grass")
	circleFill := getStringSetting(settings, "circle-fill", "clearing-grass")
	circleThickness := getIntSetting(settings, "circle-thickness", radius-1)
	centerFill := getStringSetting(settings, "center-fill", "clearing-grass")

	lx, ly, ux, uy, _, _, free := getAvailableBox(x1, y1, x2, y2, world, radius*2, radius*2)

	midx, midy := lx+(ux-lx)/2, ly+(uy-ly)/2

	circleFillBlock := CellInfo{
		TerrainID:    circleFill,
		RegionNameID: regionID}

	centerFillBlock := CellInfo{
		TerrainID:    centerFill,
		RegionNameID: regionID}

	if free {
		regionID = world.NewPlaceID()

		for x := lx; x <= ux; x++ {
			for y := ly; y <= uy; y++ {
				distanceFromCenter := int(math.Sqrt(math.Pow(float64(x-midx), 2.0) + math.Pow(float64(y-midy), 2.0)))
				if distanceFromCenter <= radius {
					if distanceFromCenter > (radius - circleThickness) {
						world.Cell(uint32(x), uint32(y)).SetCellInfo(&circleFillBlock)
					} else {
						if centerFill != "" {
							world.Cell(uint32(x), uint32(y)).SetCellInfo(&centerFillBlock)
						} else {
							world.Cell(uint32(x), uint32(y)).SetCellInfo(nil)
						}
					}
				}
			}
		}

		entryCell := CellInfo{
			TerrainID:    seedExit,
			RegionNameID: regionID}
		for rad := 0; rad < entryRadius; rad++ {
			world.Cell(uint32(midx), uint32(ly+rad)).SetCellInfo(&entryCell)
			world.Cell(uint32(midx), uint32(uy-rad)).SetCellInfo(&entryCell)
			world.Cell(uint32(lx+rad), uint32(midy)).SetCellInfo(&entryCell)
			world.Cell(uint32(ux-rad), uint32(midy)).SetCellInfo(&entryCell)
		}
	} else {
		var cellTerrain *CellTerrain

		cellInfo := world.Cell(x1, y1).CellInfo()
		if cellInfo != nil {
			cellTerrain = &cellInfo.TerrainData
		} else {
			*cellTerrain = CellTypes[seedExit]
		}
		visitSpread(x1, y1, x2, y2, world, regionID, &cellInfo.TerrainData)
	}
}

func visitChangeOfScenery(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	settings := cellTerrain.AlgorithmParameters

	length := 100
	thickness := 10
	dividerThickness := 5
	dividerEdge := "clearing-grass"
	dividerCenter := "clearing-center"
	length = getIntSetting(settings, "length", length)
	thickness = getIntSetting(settings, "thickness", thickness)
	dividerThickness = getIntSetting(settings, "divider-thickness", dividerThickness)
	dividerEdge = getStringSetting(settings, "divider-edge", dividerEdge)
	dividerCenter = getStringSetting(settings, "divider-center", dividerCenter)

	seedExit := ""
	oldInfo := seedExit
	oldCell := world.Cell(x1, y1).CellInfo()
	if oldCell != nil {
		oldInfo = oldCell.TerrainID
	}

UniqueSeedFinder:
	for x := 0; x < 5; x++ {
		seedExit = cellTerrain.GetRandomTransition()
		if seedExit != oldInfo {
			break UniqueSeedFinder
		}
	}

	width, height := thickness, length
	if y1 == y2 {
		width, height = height, width
	}

	lx, ly, _, _, xd, yd, free := getAvailableBox(x1, y1, x2, y2, world, width, height)

	if free {
		regionID := world.NewPlaceID()

		dividerCenterCell := CellInfo{RegionNameID: regionID,
			TerrainID: dividerCenter}

		// Draw mountain wall
		xp, yp := 0, 0
		if xd == 0 {
			xp = 1
		} else if yd == 0 {
			yp = 1
		}

		jitter := 0

		for l := 0; l < length; l++ {
			xc, yc := lx+(xp*l), ly+(yp*l)
			localthickness := thickness

			if l < thickness {
				localthickness = (l + 1)
			} else if l > length-thickness {
				localthickness = length - l
			}

			leftInfo, rightInfo := seedExit, oldInfo
			if xd < 0 || yd < 0 {
				leftInfo, rightInfo = rightInfo, leftInfo
			}

			for thick := 0; thick < localthickness; thick++ {
				localthick := thick - jitter

				if l == length/2 || l == length/4 || l == length/4*3 {
					if thick < dividerThickness {
						dividerCenterCell.TerrainID = rightInfo
					} else {
						dividerCenterCell.TerrainID = leftInfo
					}
				} else if localthick < 1 {
					dividerCenterCell.TerrainID = rightInfo
				} else if localthick == 1 || localthick == dividerThickness {
					dividerCenterCell.TerrainID = dividerEdge
				} else if localthick < dividerThickness {
					dividerCenterCell.TerrainID = dividerCenter
				} else if localthick < dividerThickness+2+rand.Int()%2 {
					dividerCenterCell.TerrainID = leftInfo
				} else {
					continue
				}
				world.Cell(uint32(xc+(yp*thick)), uint32(yc+(xp*thick))).SetCellInfo(&dividerCenterCell)
				jitter += (rand.Int() % 3) - 1
				if jitter < 0 {
					jitter = 0
				} else if jitter > 2 {
					jitter = 2
				}
			}
		}

		// Draw path out
		pathInfo := CellInfo{
			TerrainID:    oldInfo,
			RegionNameID: regionID}
		pathX, pathY := int(x2), int(y2)
		for t := 0; t <= thickness; t++ {
			if t > thickness/2 {
				pathInfo.TerrainID = seedExit
			}
			world.Cell(uint32(pathX), uint32(pathY)).SetCellInfo(&pathInfo)
			pathX += xd
			pathY += yd
		}
	} else {
		var cellTerrain *CellTerrain

		cellInfo := world.Cell(x1, y1).CellInfo()
		if cellInfo != nil {
			cellTerrain = &cellInfo.TerrainData
		} else {
			*cellTerrain = CellTypes[seedExit]
		}
		visitSpread(x1, y1, x2, y2, world, regionID, &cellInfo.TerrainData)
	}
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
	generationAlgorithms["change-of-scenery"] = visitChangeOfScenery
}
