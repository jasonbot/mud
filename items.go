package mud

// For the Type field of Item
const (
	ITEMTYPEWEAPON   = "Weapon"
	ITEMTYPEPOTION   = "Potion"
	ITEMTYPESCROLL   = "Scroll"
	ITEMTYPEARTIFACT = "Artifact"
)

// Weapon types for the WeaponInventoryItem
const (
	WEAPONSUBTYPESWORD  = "Sword"  // Melee
	WEAPONTYPESPEAR     = "Spear"  // Melee/Range
	WEAPONSUBTYPEDAGGER = "Dagger" // Melee/Magic
	WEAPONSUBTYPEATLATL = "Atlatl" // Range
	WEAPONSUBTYPEDART   = "Dart"   // Range/Magic
	WEAPONSUBTYPEBOW    = "Bow"    // Range/Melee
	WEAPONSUBTYPEWAND   = "Wand"   // Magic
	WEAPONSUBTYPESTAFF  = "Staff"  // Magic/Melee
	WEAPONSUBTYPEORB    = "Orb"    // Magic/Range
)

// InventoryItemHeader is a droppable item for an inventory
type InventoryItemHeader struct {
	ID   string `json:""`
	Name string `json:""`
	Type string `json:""`
}

// WeaponInventoryItem is a weapon
type WeaponInventoryItem struct {
	InventoryItemHeader
	Subtype string    `json:",omitempty"`
	Attacks []*Attack `json:",omitempty"`
}

// PotionInventoryItem is a drinkable/throwable buff/weakener
type PotionInventoryItem struct {
	InventoryItemHeader
}

// ScrollInventoryItem is a spellbook or one-off spell
type ScrollInventoryItem struct {
	InventoryItemHeader
}

// ArtifactInventoryItem is a relic with powers
type ArtifactInventoryItem struct {
	InventoryItemHeader
}

// InventoryItem is a droppable item for an inventory
type InventoryItem struct {
	InventoryItemHeader
	WeaponInventoryItem
	PotionInventoryItem
	ScrollInventoryItem
	ArtifactInventoryItem
}

// InventoryInfo handles a thing with inventory
type InventoryInfo interface {
	InventoryItems() []*InventoryItem
	AddInventoryItem(*InventoryItem) bool
	InventoryItem(string) *InventoryItem
	PullInventoryItem(string) *InventoryItem
}
