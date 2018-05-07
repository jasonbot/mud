package mud

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	bolt "github.com/coreos/bbolt"
)

// World represents a gameplay world. It should keep track of the map,
// entities in the map, and players.
type World interface {
	GetDimensions() (uint32, uint32)
	GetUser(string) User
	GetCellInfo(uint32, uint32) *CellInfo
	SetCellInfo(uint32, uint32, *CellInfo)
	NewPlaceID() uint64
	OnlineUsers() []User
	Chat(string)
	Close()
}

type dbWorld struct {
	filename string
	database *bolt.DB
}

// GetDimensions returns the size of the world
func (w *dbWorld) GetDimensions() (uint32, uint32) {
	return uint32(1 << 30), uint32(1 << 30)
}

func (w *dbWorld) GetUser(username string) User {
	return getUserFromDB(w, username)
}

func (w *dbWorld) newUser(username string) UserData {
	width, height := w.GetDimensions()
	userData := UserData{Username: username, X: width / 2, Y: height / 2, PublicKeys: make(map[string]bool)}
	cellData := w.GetCellInfo(userData.X, userData.Y)

	if cellData == nil {
		newRegionID, _ := newPlaceNameInDB(w.database)
		cellData = &CellInfo{
			TerrainType:  DefaultCellType,
			RegionNameID: newRegionID}

		w.SetCellInfo(userData.X, userData.Y, cellData)
	}

	return userData
}

func (w *dbWorld) GetCellInfo(x, y uint32) *CellInfo {
	var cellInfo CellInfo
	w.database.View(func(tx *bolt.Tx) error {

		bucket := tx.Bucket([]byte("terrain"))

		pt := Point{x, y}
		record := bucket.Get(pt.Bytes())

		if record != nil {
			cellInfo = cellInfoFromBytes(record)
		}

		return nil
	})

	placeName := getPlaceNameByIDFromDB(cellInfo.RegionNameID, w.database)
	cellTerrain, ok := CellTypes[cellInfo.TerrainType]

	if ok {
		// Format place name if it exists
		if len(cellTerrain.PlaceName) > 0 {
			placeName = fmt.Sprintf(cellTerrain.PlaceName, placeName)
		}

		cellInfo.RegionName = placeName

		return &cellInfo
	}

	return nil
}

func (w *dbWorld) SetCellInfo(x, y uint32, cellInfo *CellInfo) {
	w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("terrain"))

		pt := Point{x, y}
		bytes := cellInfoToBytes(cellInfo)
		err := bucket.Put(pt.Bytes(), bytes)

		return err
	})
}

func (w *dbWorld) NewPlaceID() uint64 {
	id, _ := newPlaceNameInDB(w.database)
	return id
}

func (w *dbWorld) OnlineUsers() []User {
	names := make([]string, 0)
	arr := make([]User, 0)
	offlineNames := make([]string, 0)

	w.database.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("onlineusers"))
		now := time.Now().UTC().Unix()

		bucket.ForEach(func(k, v []byte) error {
			var lastUpdate int64
			buf := bytes.NewBuffer(v)
			binary.Read(buf, binary.BigEndian, &lastUpdate)

			if (now - lastUpdate) < 5 {
				names = append(names, string(k))
			} else {
				offlineNames = append(offlineNames, string(k))
			}

			return nil
		})

		for _, name := range offlineNames {
			bucket.Delete([]byte(name))
		}

		return nil
	})

	for _, name := range names {
		arr = append(arr, w.GetUser(name))
	}

	for _, name := range offlineNames {
		log.Printf("%s has signed off", name)
		w.Chat(fmt.Sprintf("%s has signed off", name))
	}

	return arr
}

func (w *dbWorld) Chat(message string) {
	for _, user := range w.OnlineUsers() {
		user.Log(message)
	}
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

	// Make default tables
	db.Update(func(tx *bolt.Tx) error {
		buckets := []string{"users", "terrain", "placenames", "userlog", "onlineusers"}

		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))

			if err != nil {
				return err
			}
		}

		return nil
	})

	w.database = db
}

// LoadWorldFromDB will set up an on-disk based world
func LoadWorldFromDB(filename string) World {
	newWorld := dbWorld{filename: filename}
	newWorld.load()
	return &newWorld
}
