package mud

import (
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"

	"github.com/ojrac/opensimplex-go"
)

type tileFunc func(Cell, Cell, BiomeData, World) bool

var tileGenerationAlgorithms map[string]tileFunc

var seed *opensimplex.Noise

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

func getBoolSetting(settings map[string]string, settingName string, defaultValue bool) bool {
	if settings != nil {
		value, ok := settings[settingName]

		if ok && len(value) > 0 {
			val := strings.ToLower(value)

			if (val == "true") || (val == "1") {
				return true
			} else if (val == "false") || (val == "0") {
				return false
			}
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
	if cell.IsEmpty() {
		cell.SetCellInfo(&CellInfo{TerrainID: cellTerrain.ID, RegionNameID: regionID})
		count--
	} else {
		ci := cell.CellInfo()
		if ci.TerrainID != cellTerrain.ID {
			count--

			// Can pass through this and keep on going
			if !ci.TerrainData.Permeable {
				return
			}
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
	if tCell.IsEmpty() {
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
			if nxCell.IsEmpty() {
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

			if neighborLeft.IsEmpty() {
				neighborLeft.SetCellInfo(&CellInfo{
					TerrainID:    neighborTerrain,
					RegionNameID: regionID})
			}
			if neightborRight.IsEmpty() {
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

		if newCell.IsEmpty() {
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

func getBox(x, y uint32, direction Direction, world World, width, height uint32) (uint32, uint32, uint32, uint32, bool) {
	x1, y1, x2, y2 := x, y, x, y
	free := true

	switch direction {
	case DIRECTIONNORTH:
		x1, y1, x2, y2 = x-(width/2), y-(height-1), x-(width/2)+(width-1), y
	case DIRECTIONSOUTH:
		x1, y1, x2, y2 = x-(width/2), y, x-(width/2)+(width-1), y+(height-1)
	case DIRECTIONEAST:
		x1, y1, x2, y2 = x, y-(width/2), x+(height-1), y-(width/2)+(width-1)
	case DIRECTIONWEST:
		x1, y1, x2, y2 = x-(height-1), y-(width/2), x, y-(width/2)+(width-1)
	}

BlockCheck:
	for xc := x1; xc <= x2; xc++ {
		for yc := y1; yc <= y2; yc++ {
			if !world.Cell(uint32(xc), uint32(yc)).IsEmpty() {
				free = false
				break BlockCheck
			}
		}
	}

	return x1, y1, x2, y2, free
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
			if !cell.IsEmpty() {
				free = false
				break BlockCheck
			}
		}
	}

	return lx, ly, ux, uy, xd, yd, free
}

func snapToTile(x1, y1 uint32, tileSize int) (uint32, uint32, uint32, uint32) {
	x, y := x1, y1
	x -= x % uint32(tileSize)
	y -= y % uint32(tileSize)

	return x, y, x + uint32(tileSize), y + uint32(tileSize)
}

func getTile(x1, y1 uint32, tileSize int, world World) (uint32, uint32, uint32, uint32, bool) {
	x, y := x1, y1
	x -= x % uint32(tileSize)
	y -= y % uint32(tileSize)

	empty := true

EmptyCheck:
	for xa := x; xa <= x+uint32(tileSize); xa++ {
		for ya := y; ya <= y+uint32(tileSize); ya++ {
			if !(world.Cell(xa, ya).IsEmpty()) {
				empty = false
				break EmptyCheck
			}
		}
	}

	return x, y, x + uint32(tileSize-1), y + uint32(tileSize-1), empty
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
				if setCell.IsEmpty() {
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

func visitGreatWall(castleWall Box, world World, regionID uint64, biome BiomeData) {
	settings := biome.AlgorithmParameters

	seedExit := getStringSetting(settings, "seed-exit", "clearing-grass")

	lx, ly, ux, uy := castleWall.Coordinates()

	wallThickness := uint32(getIntSetting(settings, "wall-thickness", 3))
	seedEntry := getStringSetting(settings, "seed-entry", "gravel")
	wallTexture := getStringSetting(settings, "wall-texture", "castle-clearing-wall")
	wallTextureInfo := CellInfo{
		TerrainID:    wallTexture,
		RegionNameID: regionID}
	entryTextureInfo := CellInfo{
		TerrainID:    seedEntry,
		RegionNameID: regionID}

	floorType := getStringSetting(biome.AlgorithmParameters, "floor", "gravel")
	floor := CellInfo{
		TerrainID:    floorType,
		BiomeID:      biome.ID,
		RegionNameID: regionID}

	width, height := castleWall.WidthAndHeight()
	center := castleWall.Center()
	innerBox := BoxFromCenteraAndWidthAndHeight(&center, width-3, height-3)

	fillBox(innerBox, world, &floor)

	// Outline
	for x := uint32(0); x < (ux - lx); x++ {
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
	for y := uint32(0); y < (uy - ly); y++ {
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
	for thickness := uint32(1); thickness < wallThickness; thickness++ {
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
		RegionNameID: regionID}
	for i := uint32(0); i <= wallThickness; i++ {
		midx, midy := lx+(ux-lx)/2, ly+(uy-ly)/2
		if i >= wallThickness-1 {
			walkwayCell.TerrainID = seedEntry
		}

		world.Cell(uint32(midx), uint32(ly+i)).SetCellInfo(&walkwayCell)
		world.Cell(uint32(midx), uint32(uy-i)).SetCellInfo(&walkwayCell)
		world.Cell(uint32(lx+i), uint32(midy)).SetCellInfo(&walkwayCell)
		world.Cell(uint32(ux-i), uint32(midy)).SetCellInfo(&walkwayCell)
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

func isBoxEmpty(box Box, world World) bool {
	for x := box.TopLeft.X; x <= box.BottomRight.X; x++ {
		for y := box.TopLeft.Y; y <= box.BottomRight.Y; y++ {
			if !world.Cell(x, y).IsEmpty() {
				return false
			}
		}
	}

	return true
}

func fuzzBordersWithNeighbors(x1, y1, x2, y2 uint32, biome BiomeData, world World) {
	top, bottom := Point{X: x1, Y: y1}, Point{X: x1, Y: y2}

	width, height := int(x2-x1)/2, int(y2-y1)/2

	for xc := x1; xc <= x2; xc++ {
		topCell := world.CellAtPoint(top.Neighbor(DIRECTIONNORTH))
		bottomCell := world.CellAtPoint(bottom.Neighbor(DIRECTIONSOUTH))

		if !topCell.IsEmpty() {
			topInfo := topCell.CellInfo()
			if topInfo.BiomeID != biome.ID {
				topInfo.BiomeID = biome.ID
				pt := top

				widthFill := rand.Int() % height

				for i := 0; i < widthFill; i++ {
					cell := world.CellAtPoint(pt)
					if cell.IsEmpty() {
						cell.SetCellInfo(topInfo)
					}
					pt = pt.Neighbor(DIRECTIONSOUTH)
				}
			}
		}

		if !bottomCell.IsEmpty() {
			bottomInfo := bottomCell.CellInfo()
			if bottomInfo.BiomeID != biome.ID {
				bottomInfo.BiomeID = biome.ID
				pt := bottom

				widthFill := rand.Int() % height

				for i := 0; i < widthFill; i++ {
					cell := world.CellAtPoint(pt)
					if cell.IsEmpty() {
						cell.SetCellInfo(bottomInfo)
					}
					pt = pt.Neighbor(DIRECTIONNORTH)
				}
			}
		}

		top = top.Neighbor(DIRECTIONEAST)
		bottom = bottom.Neighbor(DIRECTIONEAST)
	}

	left, right := Point{X: x1, Y: y1}, Point{X: x2, Y: y1}
	for xc := x1; xc <= x2; xc++ {
		leftCell := world.CellAtPoint(left.Neighbor(DIRECTIONWEST))
		rightCell := world.CellAtPoint(right.Neighbor(DIRECTIONEAST))

		if !leftCell.IsEmpty() {
			leftInfo := leftCell.CellInfo()
			if leftInfo.BiomeID != biome.ID {
				leftInfo.BiomeID = biome.ID
				pt := left

				heightFill := rand.Int() % width

				for i := 0; i < heightFill; i++ {
					cell := world.CellAtPoint(pt)
					if cell.IsEmpty() {
						cell.SetCellInfo(leftInfo)
					}
					pt = pt.Neighbor(DIRECTIONEAST)
				}
			}
		}

		if !rightCell.IsEmpty() {
			rightInfo := rightCell.CellInfo()
			if rightInfo.BiomeID != biome.ID {
				rightInfo.BiomeID = biome.ID
				pt := right

				heightFill := rand.Int() % width

				for i := 0; i < heightFill; i++ {
					cell := world.CellAtPoint(pt)
					if cell.IsEmpty() {
						cell.SetCellInfo(rightInfo)
					}
					pt = pt.Neighbor(DIRECTIONWEST)
				}
			}
		}

		left = left.Neighbor(DIRECTIONSOUTH)
		right = right.Neighbor(DIRECTIONSOUTH)
	}
}

func drawBoxBorder(b Box, thickness uint32, world World, terrain *CellInfo) {
	for i := uint32(0); i < thickness; i++ {
		for x := b.TopLeft.X + i; x <= b.BottomRight.X-i; x++ {
			world.Cell(x, b.TopLeft.Y+i).SetCellInfo(terrain)
			world.Cell(x, b.BottomRight.Y-i).SetCellInfo(terrain)
		}
		for y := b.TopLeft.Y + (i + 1); y <= b.BottomRight.Y-(i+1); y++ {
			world.Cell(b.TopLeft.X+i, y).SetCellInfo(terrain)
			world.Cell(b.BottomRight.X-i, y).SetCellInfo(terrain)
		}
	}
}

func fillBox(b Box, world World, terrain *CellInfo) {
	for x := b.TopLeft.X; x <= b.BottomRight.X; x++ {
		for y := b.TopLeft.Y; y <= b.BottomRight.Y; y++ {
			world.Cell(x, y).SetCellInfo(terrain)
		}
	}
}

func fillWithNoise(x1, y1, x2, y2 uint32, biome BiomeData, terrainFunction func(float64) string, regionID uint64, world World) {
	for xc := x1; xc <= x2; xc++ {
		for yc := y1; yc <= y2; yc++ {
			cell := world.Cell(xc, yc)
			if cell.IsEmpty() {
				cell.SetCellInfo(&CellInfo{
					TerrainID: terrainFunction(
						math.Abs(
							seed.Eval2(
								float64(xc)/10.0,
								float64(yc)/10.0))),
					BiomeID:      biome.ID,
					RegionNameID: regionID})
			}
		}
	}
}

func tilePerlin(fromCell, toCell Cell, biome BiomeData, world World) bool {
	cellSize := getIntSetting(biome.AlgorithmParameters, "cell-size", 8)

	newLoc := toCell.Location()
	x1, y1, x2, y2, ok := getTile(newLoc.X, newLoc.Y, cellSize, world)

	// Fall back to filling in 8 cells around here we can if we can't get a proper block
	if !ok {
		x1, y1, x2, y2, _ = getTile(newLoc.X, newLoc.Y, 8, world)
	}

	terrains := strings.Split(biome.AlgorithmParameters["terrains"], ";")
	terrainFunction := MakeGradientTransitionFunction(terrains)

	fuzzBordersWithNeighbors(x1, y1, x2, y2, biome, world)
	fillWithNoise(x1, y1, x2, y2, biome, terrainFunction, fromCell.CellInfo().RegionNameID, world)

	spreadIfNew := getBoolSetting(biome.AlgorithmParameters, "spread-if-new", false)
	spreadNeighbors := getIntSetting(biome.AlgorithmParameters, "spread-neighbors", 0)
	if !fromCell.IsEmpty() && ((spreadIfNew && fromCell.CellInfo().BiomeID != biome.ID) || (spreadNeighbors > 0)) {
		newBox := BoxFromCoords(x1, y1, x2, y2)

		itemArr := []Box{
			newBox.Neighbor(DIRECTIONNORTH),
			newBox.Neighbor(DIRECTIONEAST),
			newBox.Neighbor(DIRECTIONSOUTH),
			newBox.Neighbor(DIRECTIONWEST)}

		directions := []Direction{DIRECTIONNORTH, DIRECTIONSOUTH, DIRECTIONEAST, DIRECTIONWEST}

		for spreadNeighbors >= 0 {
			newItems := make([]Box, 0)
			visitedTiles := make(map[Point]bool)

			for _, item := range itemArr {
				if isBoxEmpty(item, world) {
					fillWithNoise(item.TopLeft.X, item.TopLeft.Y, item.BottomRight.X, item.BottomRight.Y, biome, terrainFunction, fromCell.CellInfo().RegionNameID, world)

					for _, neighboritem := range rand.Perm(len(directions)) {
						newBox := item.Neighbor(directions[neighboritem])

						if isBoxEmpty(newBox, world) {
							_, ok := visitedTiles[newBox.TopLeft]

							if !ok {
								newItems = append(newItems, newBox)
								spreadNeighbors--
								visitedTiles[newBox.TopLeft] = true
							}
						}
					}
				}
			}

			itemArr = newItems
			if len(itemArr) == 0 {
				spreadNeighbors--
			}
		}
	}

	return true
}

func fillyReachy(fromCell, toCell Cell, biome BiomeData, world World) (Box, bool) {
	cellSize := getIntSetting(biome.AlgorithmParameters, "cell-size", 64)
	terrains := strings.Split(biome.AlgorithmParameters["terrains"], ";")

	terrainFunction := MakeGradientTransitionFunction(terrains)

	oldLoc := fromCell.Location()
	newLoc := toCell.Location()
	direction := DirectionForVector[oldLoc.Vector(newLoc)]

	x1, y1, x2, y2, ok := getBox(newLoc.X, newLoc.Y, direction, world, uint32(cellSize), uint32(cellSize))
	containerBox := BoxFromCoords(x1, y1, x2, y2)

	// Can't fill block? Fizzle out some grass.
	if !ok {
		x1, y1, x2, y2, _ = getTile(newLoc.X, newLoc.Y, 8, world)
		fuzzBordersWithNeighbors(x1, y1, x2, y2, biome, world)
		fillWithNoise(x1, y1, x2, y2, biome, terrainFunction, fromCell.CellInfo().RegionNameID, world)

		containerBox = BoxFromCoords(x1, y1, x2, y2)
		door := containerBox.Door(direction)
		nextDoor := door.Neighbor(direction)
		x1, y1, x2, y2, ok = getBox(nextDoor.X, nextDoor.Y, direction, world, uint32(cellSize), uint32(cellSize))
		if ok {
			containerBox = BoxFromCoords(x1, y1, x2, y2)
		} else {
			return containerBox, true
		}
	}

	return containerBox, false
}

func tileRuin(fromCell, toCell Cell, biome BiomeData, world World) bool {
	containerBox, handled := fillyReachy(fromCell, toCell, biome, world)

	if handled {
		return true
	}

	directions := []Direction{DIRECTIONNORTH, DIRECTIONSOUTH, DIRECTIONEAST, DIRECTIONWEST}

	c := containerBox.Center()
	x1, y1, x2, y2 := containerBox.Coordinates()

	itemArr := []Box{BoxFromCenteraAndWidthAndHeight(&c, 4, 4)}
	regionName := world.NewPlaceID()

	floorType := getStringSetting(biome.AlgorithmParameters, "floor", DefaultCellType)
	wallType := getStringSetting(biome.AlgorithmParameters, "wall", DefaultCellType)
	roomCount := getIntSetting(biome.AlgorithmParameters, "room-count", 20)
	terrains := strings.Split(biome.AlgorithmParameters["terrains"], ";")

	terrainFunction := MakeGradientTransitionFunction(terrains)

	floor := CellInfo{
		TerrainID:    floorType,
		BiomeID:      biome.ID,
		RegionNameID: regionName}

	wall := CellInfo{
		TerrainID:    wallType,
		BiomeID:      biome.ID,
		RegionNameID: regionName}

	for roomCount >= 0 {
		newItems := make([]Box, 0)
		visitedTiles := make(map[Point]bool)

		for _, item := range itemArr {
			if isBoxEmpty(item, world) {
				roomCount--
				fillBox(item, world, &floor)
				drawBoxBorder(item, 1, world, &wall)
				visitedTiles[item.TopLeft] = true

				for _, direction := range directions {
					door := item.Door(direction)
					cell := world.CellAtPoint(door.Neighbor(direction))
					ci := cell.CellInfo()
					if roomCount < 3 || ci != nil {
						world.CellAtPoint(door).SetCellInfo(&floor)

						if ci != nil && ci.TerrainID == wallType {
							cell.SetCellInfo(&floor)
						}
					}
				}

				for _, neighboritem := range rand.Perm(len(directions))[0 : 1+rand.Int()%len(directions)] {
					newBox := item.Neighbor(directions[neighboritem])
					door := item.Door(directions[neighboritem])

					if isBoxEmpty(newBox, world) {
						_, ok := visitedTiles[newBox.TopLeft]

						if !ok {
							world.CellAtPoint(door).SetCellInfo(&floor)
							newItems = append(newItems, newBox)
							visitedTiles[newBox.TopLeft] = true
						}
					}
				}
			}
		}

		itemArr = newItems
		if len(itemArr) == 0 {
			roomCount--
		}
	}

	fuzzBordersWithNeighbors(x1, y1, x2, y2, biome, world)
	fillWithNoise(x1, y1, x2, y2, biome, terrainFunction, regionName, world)

	return true
}

func tileCastle(fromCell, toCell Cell, biome BiomeData, world World) bool {
	containerBox, handled := fillyReachy(fromCell, toCell, biome, world)

	if handled {
		return true
	}

	c := containerBox.Center()
	x1, y1, x2, y2 := containerBox.Coordinates()

	regionName := world.NewPlaceID()

	terrains := strings.Split(biome.AlgorithmParameters["terrains"], ";")
	cellSize := getIntSetting(biome.AlgorithmParameters, "cell-size", 64)
	castleSize := getIntSetting(biome.AlgorithmParameters, "radius", cellSize-10)

	terrainFunction := MakeGradientTransitionFunction(terrains)

	castleWall := BoxFromCenteraAndWidthAndHeight(&c, uint32(castleSize), uint32(castleSize))

	visitGreatWall(castleWall, world, regionName, biome)

	fuzzBordersWithNeighbors(x1, y1, x2, y2, biome, world)
	fillWithNoise(x1, y1, x2, y2, biome, terrainFunction, regionName, world)

	return true
}

// PopulateCellFromAlgorithm will generate terrain
func PopulateCellFromAlgorithm(oldPos, newPos Cell, world World) bool {
	if oldPos.IsEmpty() {
		return false
	}

	if !newPos.IsEmpty() {
		return false
	}

	fixed := false

AlgoLoop:
	for i := 0; i < 25 && fixed == false; i++ {
		newBiome := oldPos.CellInfo().BiomeData.GetRandomTransition()
		biome, ok := BiomeTypes[newBiome]
		if !ok {
			biome, ok = BiomeTypes[oldPos.CellInfo().BiomeID]

			if !ok {
				return false
			}
		}

		algo, ok := tileGenerationAlgorithms[biome.Algorithm]
		if !ok {
			algo = tileGenerationAlgorithms[BiomeTypes[DefaultBiomeType].Algorithm]
		}

		if algo != nil {
			fixed = algo(oldPos, newPos, biome, world)
			if fixed {
				break AlgoLoop
			}
		} else {
			log.Printf("Nil algorithm: %v", biome.Algorithm)
		}
	}

	return fixed
}

func init() {
	seed = opensimplex.New()

	tileGenerationAlgorithms = make(map[string]tileFunc)
	tileGenerationAlgorithms["noise"] = tilePerlin
	tileGenerationAlgorithms["ruin"] = tileRuin
	tileGenerationAlgorithms["castle"] = tileCastle
}
