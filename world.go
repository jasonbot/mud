package mud

// World represents a gameplay world. It should keep track of the map,
// entities in the map, and players.
type World interface {
	GetDimensions() (uint32, uint32)
	GetUser(string) User

	GetCellInfo(uint32, uint32) *CellInfo
	SetCellInfo(uint32, uint32, *CellInfo)

	GetCreatures(uint32, uint32) []*Creature
	HasCreatures(uint32, uint32) bool
	UpdateCreature(*Creature)
	ClearCreatures(uint32, uint32)
	AddStockCreature(uint32, uint32, string)
	KillCreature(string)
	Attack(interface{}, interface{}, *Attack)

	InventoryItems(uint32, uint32) []*InventoryItem
	AddInventoryItem(uint32, uint32, *InventoryItem) bool
	InventoryItem(uint32, uint32, string) *InventoryItem
	PullInventoryItem(uint32, uint32, string) *InventoryItem
	HasInventoryItems(uint32, uint32) bool

	NewPlaceID() uint64
	OnlineUsers() []User
	Chat(LogItem)
	Close()
}
