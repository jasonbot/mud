package mud

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"strconv"
	"strings"
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

// MonsterSpawn is a JSON struct used for the generation of monsters
type MonsterSpawn struct {
	Name        string `json:""` // ID of monster in bestiary
	Probability byte   `json:""` // 0-100
	Cluster     byte   `json:""` // 1-100
}

// CellTerrain stores rules about different cell's terrain types.
// For 256 color colors check https://jonasjacek.github.io/colors/
type CellTerrain struct {
	Name                string            `json:""`
	Permeable           bool              `json:""`           // Things like paths, rivers, etc. should be permeable so biomes don't suddenly stop geneating through them.
	Blocking            bool              `json:""`           // Some terrain types are impassable; e.g. walls
	Transitions         []string          `json:""`           // Other cell types this can transition into when generating
	PlaceName           string            `json:",omitempty"` // Formatstring to modify place name
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
	TerrainType  string `json:""`
	ExitBlocks   byte   `json:""`
	RegionNameID uint64 `json:""`
	RegionName   string `json:"-"`
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

func makeTransitionFunction(name string, transitionList []string) (func() string, []string) {
	total := 0

	type transitionName struct {
		name   string
		weight int
	}

	transitionInternalList := make([]transitionName, 0)
	returnTransitionList := make([]string, 0)

	for _, transition := range transitionList {
		splitString := strings.SplitN(transition, ":", 2)
		weightString := "1"
		returnTransitionList = append(returnTransitionList, splitString[0])

		if (len(splitString)) > 1 {
			weightString = splitString[1]
		}

		weight, err := strconv.Atoi(weightString)

		if err != nil {
			panic(err)
		}

		transitionInternalList = append(transitionInternalList, transitionName{name: splitString[0], weight: weight})
		total += weight
	}

	return func() string {
		if transitionInternalList != nil {
			weight := 0
			countTo := rand.Int() % total

			for _, item := range transitionInternalList {
				weight += item.weight

				if weight > countTo {
					return item.name
				}
			}
		}
		return ""
	}, returnTransitionList
}

func init() {
	CellTypes = make(map[string]CellTerrain)

	terrainInfoFile := "./terrain.json"
	data, err := ioutil.ReadFile(terrainInfoFile)

	if err == nil {
		err = json.Unmarshal(data, &CellTypes)

		for k, val := range CellTypes {
			val.GetRandomTransition, val.Transitions = makeTransitionFunction(val.Name, val.Transitions)
			CellTypes[k] = val
		}
	}

	if err != nil {
		log.Printf("Error parsing %s: %v", terrainInfoFile, err)
	}
}
