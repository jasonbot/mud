package mud

// SlottedInventoryItem describe the slots and items in the slots
type SlottedInventoryItem struct {
	Name string
	Item *InventoryItem
}

// EquipUserInfo is for putting outfits on a user
type EquipUserInfo interface {
	Equip(string, *InventoryItem) (*InventoryItem, error)
	EquippableSlots(*InventoryItem) []string
	CanEquip(string, *InventoryItem) bool
	Equipped() []SlottedInventoryItem
	EquipmentSlotItem(string) *InventoryItem
	EquipSlots() []string
}

// EquipmentSlotInfo Decribes a user's open equipment slots
type EquipmentSlotInfo struct {
	Name      string   `json:""`
	SlotTypes []string `json:""`
}

// User represents an active user in the system.
type User interface {
	StatInfo
	StatPointable
	FullStatPointable
	ClassInfo
	LastAction
	ChargeInfo
	InventoryInfo
	EquipUserInfo

	Username() string
	Title() string
	IsInitialized() bool
	Initialize(bool)
	Location() *Point

	MoveNorth()
	MoveSouth()
	MoveEast()
	MoveWest()
	ChargePoints()

	Log(message LogItem)
	GetLog() []LogItem

	MarkActive()
	Cell() Cell
	LocationName() string

	Respawn()
	Reload()
	Save()
}

// LastAction tracks the last time an actor performed an action, for charging action bar.
type LastAction interface {
	Act()
	GetLastAction() int64
}

// ChargeInfo returns turn-base charge time info
type ChargeInfo interface {
	Charge() (int64, int64)
	Attacks() []*AttackInfo
	MusterAttack(string) *Attack
	MusterCounterAttack() *Attack
}

// UserSSHAuthentication for storing SSH auth.
type UserSSHAuthentication interface {
	SSHKeysEmpty() bool
	ValidateSSHKey(string) bool
	AddSSHKey(string)
}
