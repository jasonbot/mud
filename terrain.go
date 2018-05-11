package mud

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// DefaultCellType is the seed land type when spawning a character.
const DefaultCellType string = "clearing"

// MonsterSpawn is a JSON struct used for the generation of monsters
type MonsterSpawn struct {
	Name        string  `json:""` // ID of monster in bestiary
	Probability float32 `json:""` // 0-1.0
	Cluster     float32 `json:""` // 0-1.0
}

// CellTerrain stores rules about different cell's terrain types.
// For 256 color colors check https://jonasjacek.github.io/colors/
type CellTerrain struct {
	ID                  string            `json:""`
	Permeable           bool              `json:""`           // Things like paths, rivers, etc. should be permeable so biomes don't suddenly stop geneating through them.
	Blocking            bool              `json:""`           // Some terrain types are impassable; e.g. walls
	Transitions         []string          `json:""`           // Other cell types this can transition into when generating
	Name                string            `json:",omitempty"` // Formatstring to modify place name
	MakeNewPlaceName    bool              `json:",omitempty"` // If leaving a cell with MakeNewPlaceName:false->MakeNewPlaceName:true, generate new place name
	Algorithm           string            `json:""`           // Default is radiateout; should have algos for e.g. town grid building etc.
	AlgorithmParameters map[string]string `json:""`           // Helpers for terrain generator algorithm
	MonsterSpawns       []MonsterSpawn    `json:""`           // List of monster types and probabilities of them appearing in each terrain type
	FGcolor             byte              `json:""`           // SSH-display specific: the 256 color xterm color for FG
	BGcolor             byte              `json:""`           // SSH-display specific: the 256 color xterm color for BG
	Bold                bool              `json:""`           // SSH-display specific: bold the cell FG?
	Representations     []rune            `json:""`           // SSH-display specific: unicode chars to use to represent this cell on-screen
	GetRandomTransition func() string     // What to transition to
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
	TerrainID    string      `json:""`
	TerrainData  CellTerrain `json:"-"`
	ExitBlocks   byte        `json:""`
	RegionNameID uint64      `json:""`
	RegionName   string      `json:"-"`
}

// cellInfoFromBytes reads a CellInfo from raw bytes
func cellInfoFromBytes(data []byte) CellInfo {
	var cellInfo CellInfo
	json.Unmarshal(data, &cellInfo)
	return cellInfo
}

// cellInfoToBytes reads a CellInfo to JSON
func cellInfoToBytes(cellInfo *CellInfo) []byte {
	data, _ := json.Marshal(cellInfo)
	return data
}

func loadTerrainTypes(terrainInfoFile string) {
	data, err := ioutil.ReadFile(terrainInfoFile)

	if err == nil {
		err = json.Unmarshal(data, &CellTypes)

		for k, val := range CellTypes {
			val.ID = k
			val.GetRandomTransition, val.Transitions = MakeTransitionFunction(val.ID, val.Transitions)
			CellTypes[k] = val
		}
	}

	if err != nil {
		log.Printf("Error parsing %s: %v", terrainInfoFile, err)
	}
}

func init() {
	CellTypes = make(map[string]CellTerrain)
}
