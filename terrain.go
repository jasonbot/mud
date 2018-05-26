package mud

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// DefaultBiomeType yes
const DefaultBiomeType = "open-grass"

// BiomeData contains information about biome types
type BiomeData struct {
	ID                  string
	Name                string            `json:""`
	Algorithm           string            `json:""` // Need strategies to make land
	AlgorithmParameters map[string]string `json:""` // Helpers for terrain generator algorithm
	Transitions         []string          `json:""` // Other biome types this can transition into when generating
	GetRandomTransition func() string     // What to transition to
}

// DefaultCellType is the seed land type when spawning a character.
const DefaultCellType string = "clearing"

// CellTerrain stores rules about different cell's terrain types.
// For 256 color colors check https://jonasjacek.github.io/colors/
type CellTerrain struct {
	ID                  string            `json:""`
	Permeable           bool              `json:""`           // Things like paths, rivers, etc. should be permeable so biomes don't suddenly stop geneating through them.
	Blocking            bool              `json:""`           // Some terrain types are impassable; e.g. walls
	Name                string            `json:",omitempty"` // Formatstring to modify place name
	Algorithm           string            `json:""`           // Should have algos for e.g. town grid building etc.
	AlgorithmParameters map[string]string `json:""`           // Helpers for terrain generator algorithm
	CreatureSpawns      []CreatureSpawn   `json:""`           // List of monster types and probabilities of them appearing in each terrain type
	ItemDrops           []ItemDrop        `json:""`           // List of items and probabilities of them appearing in each terrain type
	FGcolor             byte              `json:""`           // SSH-display specific: the 256 color xterm color for FG
	BGcolor             byte              `json:""`           // SSH-display specific: the 256 color xterm color for BG
	Bold                bool              `json:""`           // SSH-display specific: bold the cell FG?
	Animated            bool              `json:""`           // SSH-display specific: Fake an animation effect?
	Representations     []rune            `json:""`           // SSH-display specific: unicode chars to use to represent this cell on-screen
}

// CellTypes is the list of cell types
var CellTypes map[string]CellTerrain

// BiomeTypes is the list of cell types
var BiomeTypes map[string]BiomeData

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
	BiomeID      string      `json:""`
	BiomeData    BiomeData   `json:"-"`
	ExitBlocks   byte        `json:""`
	RegionNameID uint64      `json:""`
	RegionName   string      `json:"-"`
}

func loadTerrainTypes(terrainInfoFile string) {
	data, err := ioutil.ReadFile(terrainInfoFile)

	var terrainFileData struct {
		CellTypes  map[string]CellTerrain `json:"cells"`
		BiomeTypes map[string]BiomeData   `json:"biomes"`
	}

	if err == nil {
		err = json.Unmarshal(data, &terrainFileData)

		BiomeTypes = make(map[string]BiomeData)
		for k, val := range terrainFileData.BiomeTypes {
			val.ID = k
			val.GetRandomTransition, val.Transitions = MakeTransitionFunction(val.ID, val.Transitions)
			BiomeTypes[k] = val
		}

		CellTypes = make(map[string]CellTerrain)
		for k, val := range terrainFileData.CellTypes {
			val.ID = k
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
