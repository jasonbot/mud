package mud

import (
	"log"

	bolt "github.com/coreos/bbolt"
)

// World represents a gameplay world. It should keep track of the map,
// entities in the map, and players.
type World interface {
	GetDimensions() (uint32, uint32)
	GetUser(string) User
	GetCellInfo(uint32, uint32) CellInfo
	SetCellInfo(uint32, uint32, CellInfo)
	GetTerrainMap(uint32, uint32, uint32, uint32) [][]CellTerrain
	Close()
}

type dbWorld struct {
	filename string
	database *bolt.DB
}

// GetDimensions returns the size of the world
func (w *dbWorld) GetDimensions() (uint32, uint32) {
	return uint32(1 << 31), uint32(1 << 31)
}

func (w *dbWorld) GetUser(username string) User {
	return getUserFromDB(w, username)
}

func (w *dbWorld) newUser(username string) UserData {
	return UserData{username: username, x: 1024, y: 1024}
}

func (w *dbWorld) GetCellInfo(x, y uint32) CellInfo {
	var cellInfo CellInfo
	w.database.View(func(tx *bolt.Tx) error {

		bucket, err := tx.CreateBucketIfNotExists([]byte("terrain"))

		if err != nil {
			return err
		}

		pt := Point{x, y}
		record := bucket.Get(pt.Bytes())

		if record != nil {
			cellInfo = CellInfoFromBytes(record)
		}

		return nil
	})

	return cellInfo
}

func (w *dbWorld) SetCellInfo(uint32, uint32, CellInfo) {

}

func (w *dbWorld) GetTerrainMap(x1, y1, x2, y2 uint32) [][]CellTerrain {
	terrainMap := make([][]CellTerrain, x2-x1)
	for i := range terrainMap {
		terrainMap[i] = make([]CellTerrain, y2-y1)
	}
	return terrainMap
}

func (w *dbWorld) Close() {
	if w.database != nil {
		w.database.Close()
	}
}

func (w *dbWorld) load() {
	log.Printf("Loading world database %s", w.filename)
	db, err := bolt.Open(w.filename, 0600, nil)

	if err != nil {
		panic(err)
	}

	w.database = db
}

// LoadWorldFromDB will set up an on-disk based world
func LoadWorldFromDB(filename string) World {
	newWorld := dbWorld{filename: filename}
	newWorld.load()
	return &newWorld
}
