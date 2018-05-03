package mud

// World represents a gameplay world. It should keep track of the map,
// entities in the map, and players.
type World interface {
	GetDimensions() (uint32, uint32)
}

type dbWorld struct {
	filename string
}

// GetDimensions returns the size of the world
func (w *dbWorld) GetDimensions() (uint32, uint32) {
	return uint32(1 << 31), uint32(1 << 31)
}

// LoadWorldFromDB will set up an on-disk based world
func LoadWorldFromDB(filename string) World {
	newWorld := dbWorld{filename: filename}
	return &newWorld
}
