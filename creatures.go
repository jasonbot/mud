package mud

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// CreatureTypes is a mapping of string IDs to creature types
var CreatureTypes map[string]CreatureType

// CreatureSpawn is a JSON struct used for the generation of monsters
type CreatureSpawn struct {
	Name        string  `json:""` // ID of creature in bestiary
	Probability float32 `json:""` // 0-1.0
	Cluster     float32 `json:""` // 0-1.0
}

// CreatureType is the type of creature (Hostile: true is monster, false is NPC)
type CreatureType struct {
	ID        string     `json:"-"`
	Name      string     `json:""`
	Hostile   bool       `json:""`
	MaxHP     uint64     `json:""`
	MaxMP     uint64     `json:""`
	MaxAP     uint64     `json:""`
	MaxRP     uint64     `json:""`
	Attacks   []Attack   `json:""`
	ItemDrops []ItemDrop `json:""` // List of items and probabilities of them appearing in each terrain type
}

// Creature is an instance of a Creature
type Creature struct {
	ID                 string       `json:""`
	CreatureType       string       `json:""`
	X                  uint32       `json:""`
	Y                  uint32       `json:""`
	HP                 uint64       `json:""`
	AP                 uint64       `json:""`
	RP                 uint64       `json:""`
	MP                 uint64       `json:""`
	CreatureTypeStruct CreatureType `json:"-"`
	Charge             int64        `json:"-"`
	maxCharge          int64
	world              World
}

// StatPoints is for StatPointable
func (creature *Creature) StatPoints() StatPoints {
	return StatPoints{
		AP: creature.CreatureTypeStruct.MaxAP,
		RP: creature.CreatureTypeStruct.MaxRP,
		MP: creature.CreatureTypeStruct.MaxMP}
}

// CreatureList represents the creatures in a DB
type CreatureList struct {
	CreatureIDs []string `json:""`
}

func loadCreatureTypes(creatureInfoFile string) {
	data, err := ioutil.ReadFile(creatureInfoFile)

	if err == nil {
		err = json.Unmarshal(data, &CreatureTypes)
	}

	for k, v := range CreatureTypes {
		v.ID = k
		CreatureTypes[k] = v
	}

	if err != nil {
		log.Printf("Error parsing %s: %v", creatureInfoFile, err)
	}
}

func init() {
	CreatureTypes = make(map[string]CreatureType)
}
