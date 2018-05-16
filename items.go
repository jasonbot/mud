package mud

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

// ItemTypes is a mapping of string item names to item types
var ItemTypes map[string]InventoryItem

// ItemDrop is a JSON struct used for the generation of random drops
type ItemDrop struct {
	Name        string  `json:""` // Name of item in items.json
	Cluster     uint    `json:""` // 0-10000
	Probability float32 `json:""` // 0-1.0
}

// For the Type field of Item
const (
	ITEMTYPEWEAPON   = "Weapon"
	ITEMTYPEPOTION   = "Potion"
	ITEMTYPESCROLL   = "Scroll"
	ITEMTYPEARTIFACT = "Artifact"
)

// Weapon types for the WeaponInventoryItem
const (
	WEAPONSUBTYPESWORD   = "Sword"   // Melee
	WEAPONTYPESPEAR      = "Spear"   // Melee/Range
	WEAPONSUBTYPEDAGGER  = "Dagger"  // Melee/Magic
	WEAPONSUBTYPEBOW     = "Bow"     // Range
	WEAPONSUBTYPEDART    = "Dart"    // Range/Magic
	WEAPONSUBTYPEJAVELIN = "Javelin" // Range/Melee
	WEAPONSUBTYPEWAND    = "Wand"    // Magic
	WEAPONSUBTYPESTAFF   = "Staff"   // Magic/Melee
	WEAPONSUBTYPEORB     = "Orb"     // Magic/Range
)

// Artifact types for the ArtifactInventoryItem
const (
	ARTIFACTTYPEAMULET     = "Amulet"
	ARTIFACTTYPERELIC      = "Relic"
	ARTIFACTTYPECURIOSITY  = "Curiosity"
	ARTIFACTTYPEINGREDIENT = "Ingredient"
)

// InventoryItem is a droppable item for an inventory
type InventoryItem struct {
	ID             string   `json:""`
	Name           string   `json:""`
	Type           string   `json:""`
	Description    string   `json:""`
	Subtype        string   `json:",omitempty"` // For weapons and artifacts
	Attacks        []Attack `json:",omitempty"` // For weapons and spells
	CounterAttacks []Attack `json:",omitempty"` // For scrolls and spells with counterattack effects
}

// InventoryInfo handles a thing with inventory
type InventoryInfo interface {
	InventoryItems() []*InventoryItem
	AddInventoryItem(*InventoryItem) bool
	InventoryItem(string) *InventoryItem
	PullInventoryItem(string) *InventoryItem
}

func loadItemTypes(itemInfoFile string) {
	data, err := ioutil.ReadFile(itemInfoFile)

	if err == nil {
		err = json.Unmarshal(data, &ItemTypes)
	}

	for k, v := range ItemTypes {
		v.Name = k
		ItemTypes[k] = v
	}

	if err != nil {
		log.Printf("Error parsing %s: %v", itemInfoFile, err)
	}
}

func init() {
	ItemTypes = make(map[string]InventoryItem)
}
