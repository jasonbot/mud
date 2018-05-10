package mud

import (
	"math/rand"
	"strconv"
)

type visitFunc func(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain)

var generationAlgorithms map[string]visitFunc

var defaultAlgorithm = "once"

func visitOnce(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	world.SetCellInfo(x2, y2, &CellInfo{TerrainType: cellTerrain.Name, RegionNameID: regionID})
}

func tendril(x, y uint32, count uint64, world World, regionID uint64, cellTerrain *CellTerrain) {
	if count <= 0 {
		return
	}

	cell := world.GetCellInfo(x, y)
	if cell == nil {
		world.SetCellInfo(x, y, &CellInfo{TerrainType: cellTerrain.Name, RegionNameID: regionID})
		count--
	} else if cell.TerrainType != cellTerrain.Name {
		k, ok := CellTypes[cell.TerrainType]
		count--

		// Can pass through this and keep on going
		if (ok) && k.Permeable {
			return
		}
	}

	width, height := world.GetDimensions()
	if x > 0 && y > 0 && x < width-2 && y < height-2 {
		nx, ny := x, y
		for nx == x && ny == y {
			nx, ny = uint32(int(x)+rand.Int()%3-1), uint32(int(y)+rand.Int()%3-1)
		}
		tendril(nx, ny, count, world, regionID, cellTerrain)
	}
}

func visitTendril(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	radius := uint64(4)
	tendrilcount := uint64(0)
	if cellTerrain.AlgorithmParameters != nil {
		radiusString, ok := cellTerrain.AlgorithmParameters["radius"]
		if ok {
			radiusI, err := strconv.Atoi(radiusString)

			if err != nil {
				radius = uint64(radiusI)
			}
		}

		tendrilcount = radius
		tendrilcountString, ok := cellTerrain.AlgorithmParameters["tendrilcount"]
		if ok {
			tendrilcountI, err := strconv.Atoi(tendrilcountString)

			if err != nil {
				tendrilcount = uint64(tendrilcountI)
			}
		}
	}

	for i := 0; i < int(tendrilcount); i++ {
		tendril(x2, y2, radius, world, regionID, cellTerrain)
	}

	for xd := -1; xd < 2; xd++ {
		for yd := -1; yd < 2; yd++ {
			nx, ny := uint32(int(x2)+xd), uint32(int(y2)+yd)
			ci := world.GetCellInfo(nx, ny)
			if ci == nil {
				world.SetCellInfo(nx, ny, &CellInfo{TerrainType: cellTerrain.Name, RegionNameID: regionID})
			}
		}
	}
}

func visitSpread(x1, y1, x2, y2 uint32, world World, regionID uint64, cellTerrain *CellTerrain) {
	blocked := false

	world.SetCellInfo(x2, y2, &CellInfo{TerrainType: cellTerrain.Name, RegionNameID: regionID})

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
				world.SetCellInfo(nx, ny, &CellInfo{TerrainType: cellTerrain.Name, RegionNameID: regionID})
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
	radiusString, radiusok := cellTerrain.AlgorithmParameters["radius"]
	radius := uint64(5)

	world.SetCellInfo(x1, y1,
		&CellInfo{
			TerrainType:  cellTerrain.Name,
			RegionNameID: regionID})

	if radiusok {
		radiusI, err := strconv.Atoi(radiusString)

		if err != nil {
			radius = uint64(radiusI)
		}
	}

	if !ok {
		ci := world.GetCellInfo(uint32(int(x1)+(xd*-2)), uint32(int(y1)+(yd*-2)))
		if ci != nil {
			neighborTerrain = ci.TerrainType
		}
	}

	length := int(radius/2) + rand.Int()%int(radius/2)
	broken := false

	for i := 0; i < length; i++ {
		newCell := world.GetCellInfo(uint32(nx), uint32(ny))

		if newCell == nil || newCell.TerrainType == neighborTerrain {
			world.SetCellInfo(uint32(nx), uint32(ny),
				&CellInfo{
					TerrainType:  cellTerrain.Name,
					RegionNameID: regionID})

			neighborLeft := world.GetCellInfo(uint32(nx+yd), uint32(ny+xd))
			neightborRight := world.GetCellInfo(uint32(nx-yd), uint32(ny-xd))

			if neighborLeft == nil {
				world.SetCellInfo(uint32(nx+yd), uint32(ny+xd),
					&CellInfo{
						TerrainType:  neighborTerrain,
						RegionNameID: regionID})
			}
			if neightborRight == nil {
				world.SetCellInfo(uint32(nx-yd), uint32(ny-xd),
					&CellInfo{
						TerrainType:  neighborTerrain,
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
			world.SetCellInfo(uint32(nx), uint32(ny), &CellInfo{TerrainType: endcap, RegionNameID: regionID})

			if rand.Int()%3 > 0 {
				visitPath(uint32(nx), uint32(ny), uint32(nx+1), uint32(ny), world, regionID, cellTerrain)
				visitPath(uint32(nx), uint32(ny), uint32(nx-1), uint32(ny), world, regionID, cellTerrain)
				visitPath(uint32(nx), uint32(ny+1), uint32(nx), uint32(ny), world, regionID, cellTerrain)
				visitPath(uint32(nx), uint32(ny-1), uint32(nx), uint32(ny), world, regionID, cellTerrain)
			}
		}
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
}
