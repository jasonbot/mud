package mud

import (
	"math/rand"
	"strings"
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
func RandomPlaceName() string {
	name := ""
	for w := 0; w < 1+rand.Int()%2; w++ {
		if len(name) > 0 {
			name += " "
		}
		if rand.Int()%2 == 0 {
			if rand.Int()%2 == 0 {
				name += prefixes[rand.Int()%len(prefixes)]
			}
			for i := 0; i < 1+rand.Int()%2; i++ {
				name += randomOnset() + randomRhyme(i > 0)
			}
			name += suffixes[rand.Int()%len(suffixes)]
		} else {
			name += randomName()
		}
	}

	return strings.Title(name)
}

func init() {
	onsets = []string{"s", "sp", "spr", "spl", "th", "z", "g", "gr", "n", "m"}
	nucleae = []string{"en", "em", "ul", "er", "il", "po", "to"}
	vowels = []string{"a", "i", "u", "e", "o"}
	codae = []string{"p", "t", "k", "f", "s", "sh", "os", "ers", ""}
	prefixes = []string{"penrhyn", "sir", "newydd", "pant", "new ", "old ", "den", "high", "ast", "black", "white", "green", "castle", "heck", "hell", "button", "glen", "myr", "griffin", "lion", "bear", "pegasus", "corn"}
	middles = []string{"helms", "al", "ox", "horse", "tree", "stone", "men", "fond", "muck", "cross", "snake", "", ""}
	suffixes = []string{"fill", "sley", "well", "stone", "wich", "ddych", "thorpe", "den", "ton", "chester", "worth", "land", "hole", "park", " hole", " corner", " bend", " place", " mawr"}
}
