package mud

import "math/rand"

type visitFunc func(x1, y1, x2, y2 uint32, world World, cellTerrain *CellTerrain)

var generationAlgorithms map[string]visitFunc

var defaultAlgorithm = "once"

func visitOnce(x1, y1, x2, y2 uint32, world World, cellTerrain *CellTerrain) {
	oldCell := world.GetCellInfo(x1, y1)
	var regionID uint64
	if oldCell != nil {
		regionID = oldCell.RegionNameID
	} else {
		regionID = world.NewPlaceID()
	}
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

func visitSpread(x1, y1, x2, y2 uint32, world World, cellTerrain *CellTerrain) {
	oldCell := world.GetCellInfo(x1, y1)
	var regionID uint64
	if oldCell != nil {
		regionID = oldCell.RegionNameID
	} else {
		regionID = world.NewPlaceID()
	}

	for i := 0; i < 10; i++ {
		tendril(x2, y2, 4, world, regionID, cellTerrain)
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

// PopulateCellFromAlgorithm will run the specified algorithm to generate terrain
func PopulateCellFromAlgorithm(x1, y1, x2, y2 uint32, world World, cellTerrain *CellTerrain) {
	if cellTerrain == nil {
		return
	}

	algo, ok := generationAlgorithms[cellTerrain.Algorithm]

	if !ok {
		algo = generationAlgorithms["once"]
	}

	algo(x1, y1, x2, y2, world, cellTerrain)
}

func init() {
	generationAlgorithms = make(map[string]visitFunc)

	generationAlgorithms["once"] = visitOnce
	generationAlgorithms["spread"] = visitSpread
}
