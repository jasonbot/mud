package mud

import (
	"math/rand"
	"strings"
)

var onsets, vowels, nucleae, codae []string

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

// RandomPlaceName generates a random place name
func RandomPlaceName() string {
	name := ""
	for w := 0; w < 1+rand.Int()%2; w++ {
		if len(name) > 0 {
			name += " "
		}
		for i := 0; i < 1+rand.Int()%2; i++ {
			name += randomOnset() + randomRhyme(i > 0)
		}
	}

	return strings.Title(name)
}

func init() {
	onsets = []string{"b", "d", "g", "m", "n", "v", "th", "l", "r", "h", "zh", "y"}
	nucleae = []string{"en", "em", "ul", "er", "ilh", "po", "to"}
	vowels = []string{"a", "i", "u", "e", "o"}
	codae = []string{"p", "t", "k", "f", "s", "sh", ""}
}
