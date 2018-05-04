package mud

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"log"
)

// Point represents an (X,Y) pair in the world
type Point struct {
	X uint32
	Y uint32
}

// Bytes Dumps a point into a byte array
func (p *Point) Bytes() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, p)
	return buf.Bytes()
}

// PointFromBytes rehydrates a point struct
func PointFromBytes(ptBytes []byte) Point {
	var pt Point
	buf := bytes.NewBuffer(ptBytes)
	binary.Read(buf, binary.LittleEndian, &pt)
	return pt
}

// DefaultCellType is the seed land type when spawning a character.
const DefaultCellType string = "clearing"

// CellTerrain stores rules about different cell's terrain types.
// For 256 color colors check https://jonasjacek.github.io/colors/
type CellTerrain struct {
	Name string `json:""`
	// Things like paths, rivers, etc. should be permeable so biomes don't suddenly stop geneating through them.
	Permeable bool `json:""`
	// Some terrain types are impassable; e.g. walls
	Blocking        bool
	Transitions     []string `json:""` // Other cell types this can transition into when generating
	Radius          uint16   `json:""` // How far out to go; default of 0 should be significant somehow
	Algorithm       string   `json:""` // Default is radiateout; should have algos for e.g. town grid building etc.
	FGcolor         byte     `json:""` // SSH-display specific: the 256 color xterm color for FG
	BGcolor         byte     `json:""` // SSH-display specific: the 256 color xterm color for BG
	Bold            bool     `json:""` // SSH-display specific: bold the cell FG?
	Representations []rune   `json:""` // SSH-display specific: unicode chars to use to represent this cell on-screen
}

// CellTypes is the list of cell types
var CellTypes map[string]CellTerrain

// NORTHBIT North for bitmasks
// EASTBIT  East for bitmasks
// SOUTHBIT South for bitmasks
// WESTBIT  West for bitmasks
const (
	NORTHBIT = 1
	EASTBIT  = 2
	SOUTHBIT = 4
	WESTBIT  = 8
)

// CellInfo holds more information on the cell: exits, items available, etc.
type CellInfo struct {
	TerrainType  string `json:""`
	Exits        byte   `json:""`
	RegionNameID uint64 `json:""`
	RegionName   string `json:"-"`
}

// CellInfoFromBytes reads a CellInfo from raw bytes
func CellInfoFromBytes(data []byte) CellInfo {
	var cellInfo CellInfo
	json.Unmarshal(data, &cellInfo)
	return cellInfo
}

func init() {
	CellTypes = make(map[string]CellTerrain)

	terrainInfoFile := "./terrain.json"
	data, err := ioutil.ReadFile(terrainInfoFile)

	if err == nil {
		err = json.Unmarshal(data, &CellTypes)
	}

	if err != nil {
		log.Printf("Terrain info file %s errored: %v; using bad defaults.", terrainInfoFile, err)

		CellTypes[DefaultCellType] = CellTerrain{
			Name:            "Clearing of %s",
			Radius:          1,
			FGcolor:         184,
			BGcolor:         0,
			Transitions:     []string{DefaultCellType + "-grass"},
			Representations: []rune{rune('+')}}

		CellTypes[DefaultCellType+"-grass"] = CellTerrain{
			Name:            "%s grasslands",
			Radius:          10,
			FGcolor:         112,
			BGcolor:         154,
			Transitions:     []string{DefaultCellType + "-grass"},
			Representations: []rune{rune('⺾'), rune('艸'), rune('草')}}

		outBytes, _ := json.Marshal(CellTypes)

		ioutil.WriteFile(terrainInfoFile, outBytes, 0611)

		return
	}
}
