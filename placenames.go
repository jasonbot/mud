package mud

import (
	"encoding/binary"
	"math/rand"
	"strings"

	bolt "github.com/coreos/bbolt"
)

var onsets, vowels, nucleae, codae, prefixes, middles, suffixes []string

func randomOnset() string {
	if rand.Int()%2 == 0 {
		return randomVowel()
	}
	return onsets[rand.Int()%len(onsets)]
}

func randomNucleus() string {
	return nucleae[rand.Int()%len(nucleae)]
}

func randomVowel() string {
	return vowels[rand.Int()%len(vowels)]
}

func randomCoda() string {
	return codae[rand.Int()%len(codae)]
}

func randomRhyme(inWord bool) string {
	if inWord && rand.Int()%4 == 0 {
		return randomNucleus()
	} else if rand.Int()%4 == 0 {
		return randomVowel() + randomCoda() + randomVowel()
	}
	return randomVowel() + randomCoda()
}

func randomName() string {
	return prefixes[rand.Int()%len(prefixes)] + middles[rand.Int()%len(middles)] + suffixes[rand.Int()%len(suffixes)]
}

// RandomPlaceName generates a random place name
func randomPlaceName() string {
	name := ""
	for w := 0; w < 1+rand.Int()%2; w++ {
		if len(name) > 0 {
			name += " "
		}
		if rand.Int()%2 == 0 {
			noPrefix := true
			if rand.Int()%2 == 0 {
				noPrefix = false
				name += prefixes[rand.Int()%len(prefixes)]
			}
			for i := 0; i < 1+rand.Int()%2; i++ {
				name += randomOnset() + randomRhyme(i > 0)
			}
			if rand.Int()%2 == 0 || noPrefix {
				name += suffixes[rand.Int()%len(suffixes)]
			}
		} else {
			name += randomName()
		}
	}

	if len(name) > 25 {
		return randomPlaceName()
	}

	return strings.Title(name)
}

func newPlaceNameInDB(db *bolt.DB) (uint64, string) {
	var id uint64
	placeName := randomPlaceName()

	db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("placenames"))

		var err error
		id, err = bucket.NextSequence()

		if err != nil {
			return err
		}

		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(id))
		err = bucket.Put(b, []byte(placeName))

		return err
	})

	return id, placeName
}

func getPlaceNameByIDFromDB(id uint64, db *bolt.DB) string {
	placeName := "Delaware"

	db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("placenames"))
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, uint64(id))
		record := bucket.Get(b)

		if record != nil {
			placeName = string(record)
		}

		return nil
	})

	return placeName
}

func init() {
	onsets = []string{"s", "sp", "spr", "spl", "th", "z", "g", "gr", "n", "m"}
	nucleae = []string{"en", "em", "ul", "er", "il", "po", "to"}
	vowels = []string{"a", "i", "u", "e", "o"}
	codae = []string{"p", "t", "k", "f", "s", "sh", "os", "ers", ""}
	prefixes = []string{"nor", "sur", "wess", "ess", "jer", "hamp", "penrhyn", "trans", "mid", "man", "men", "sir", "dun", "beas", "newydd", "pant", "new ", "old ", "den", "high", "ast", "black", "white", "green", "castle", "heck", "hell", "button", "glen", "myr", "griffin", "lion", "bear", "pegasus", "sheep", "goat", "grouse", "pelican", "gull", "sparrow", "hawks", "starling", "badger", "otter", "tiger", "goose", "hogs", "hedgehog", "mouse", "shields", "swords", "spears", "cloaks", "gloven", "circus", "corn", "gren"}
	middles = []string{"helms", "al", "ox", "horse", "tree", "sylvania", "stone", "men", "fond", "muck", "cross", "snake", "yank", "her", "dam", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""}
	suffixes = []string{"fill", "sley", "sey", "spey", "well", "stone", "wich", "ddych", "thorpe", "den", "ton", "chester", "worth", "land", "hole", "park", "ware", "ine", "pile", "ina", "feld", "hoff", "wind", "dal", "hope", "kirk", "cen", "eux", "ans", "mont", "noble", "hole", "corner", "bend", "place", "mawr", "circle", "square", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", "", ""}
}
