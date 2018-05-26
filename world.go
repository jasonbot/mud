package mud

// Cell represents the data about a living cell
type Cell interface {
	Location() Point
	IsEmpty() bool
	CellInfo() *CellInfo
	SetCellInfo(*CellInfo)

	GetCreatures() []*Creature
	HasCreatures() bool
	UpdateCreature(*Creature)
	ClearCreatures()
	AddStockCreature(string)

	InventoryItems() []*InventoryItem
	AddInventoryItem(*InventoryItem) bool
	InventoryItem(string) *InventoryItem
	PullInventoryItem(string) *InventoryItem
	HasInventoryItems() bool
}

// World represents a gameplay world. It should keep track of the map,
// entities in the map, and players.
type World interface {
	GetDimensions() (uint32, uint32)
	GetUser(string) User

	Cell(uint32, uint32) Cell
	KillCreature(string)
	Attack(interface{}, interface{}, *Attack)

	NewPlaceID() uint64
	OnlineUsers() []User
	Chat(LogItem)
	Close()
}
