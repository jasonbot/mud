package mud

// EquipUserInfo is for putting outfits on a user
type EquipUserInfo interface {
	Equip(*InventoryItem) (*InventoryItem, error)
	Equipped() []*InventoryItem
}

// User represents an active user in the system.
type User interface {
	StatInfo
	ClassInfo
	LastAction
	ChargeInfo
	InventoryInfo
	EquipUserInfo

	Username() string
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
