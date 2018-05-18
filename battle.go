package mud

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
)

// StatInfo handles user/NPC stats
type StatInfo interface {
	HP() uint64
	SetHP(uint64)
	MP() uint64
	SetMP(uint64)
	AP() uint64
	SetAP(uint64)
	RP() uint64
	SetRP(uint64)
	MaxHP() uint64
	SetMaxHP(uint64)
	MaxMP() uint64
	SetMaxMP(uint64)
	MaxAP() uint64
	SetMaxAP(uint64)
	MaxRP() uint64
	SetMaxRP(uint64)
	XP() uint64
	AddXP(uint64)
	XPToNextLevel() uint64
}

// GetStatPoints is for StatPointable
func GetStatPoints(statinfo StatInfo) StatPoints {
	return StatPoints{
		AP: statinfo.MaxAP(),
		RP: statinfo.MaxRP(),
		MP: statinfo.MaxMP()}
}

// StatPointable lets an item return StatInfos for battle calculations
type StatPointable interface {
	StatPoints() StatPoints
}

// FullStatPointable is more or less the same thing
type FullStatPointable interface {
	FullStatPoints() FullStatPoints
}

// StatPoints is a passable struct for managing attack/defense calculations
type StatPoints struct {
	AP uint64
	RP uint64
	MP uint64
}

// FullStatPoints is a passable struct for attack/defense that does EVERYTHING.
type FullStatPoints struct {
	StatPoints
	Trample uint64
	HP      uint64
}

// ApplyDefense takes two StatPoints to find how much attack points apply
func (input *StatPoints) ApplyDefense(apply *StatPoints) *StatPoints {
	if input == nil || apply == nil {
		return nil
	}

	resolved := StatPoints{
		AP: input.AP,
		RP: input.RP,
		MP: input.MP}

	if input.AP <= apply.RP {
		resolved.AP = 0
	} else {
		resolved.AP -= apply.RP
	}

	if input.RP <= apply.MP {
		resolved.RP = 0
	} else {
		resolved.RP -= apply.MP
	}

	if input.MP <= apply.AP {
		resolved.MP = 0
	} else {
		resolved.MP -= apply.AP
	}

	return &resolved
}

// Damage is how much untyped damage is dealt via this StatPoints
func (input *StatPoints) Damage() uint64 {
	return input.AP + input.MP + input.RP
}

// Attack is a type of attack a creature can inflict
type Attack struct {
	ID           string   `json:",omitempty"`
	Name         string   `json:""`
	Accuracy     byte     `json:""`
	MP           uint64   `json:""`
	AP           uint64   `json:""`
	RP           uint64   `json:""`
	Trample      uint64   `json:""`
	Bonuses      string   `json:""`
	UsesItems    []string `json:",omitempty"`
	OutputsItems []string `json:",omitempty"`
	Effects      []string `json:""`
	Charge       int64    `json:""` // In Seconds
}

func (atk *Attack) String() string {
	return fmt.Sprintf("%v: AP:%v RP:%v MP:%v", atk.Name, atk.AP, atk.RP, atk.MP)
}

// StatPoints gets a stripped down statinfo object for battle calculation arithmetic
func (atk *Attack) StatPoints() StatPoints {
	return StatPoints{
		AP: atk.AP,
		RP: atk.RP,
		MP: atk.MP}
}

// FullStatPoints gets a fullstatinfo object for battle calculation arithmetic
func (atk *Attack) FullStatPoints() FullStatPoints {
	return FullStatPoints{
		StatPoints: StatPoints{
			AP: atk.AP,
			RP: atk.RP,
			MP: atk.MP},
		Trample: atk.Trample}
}

func (atk *Attack) applyStatPoints(sp FullStatPoints) Attack {
	newAtk := *atk

	newAtk.AP = sp.AP
	newAtk.RP = sp.AP
	newAtk.MP = sp.MP
	newAtk.Trample = sp.Trample

	return newAtk
}

// ApplyBonuses Apply any bonuses to attack stats based on bonus string (TODO)
func (atk *Attack) ApplyBonuses(sp FullStatPointable) Attack {
	statSP := sp.FullStatPoints()
	atkSP := atk.FullStatPoints()

	newStats := ApplyBonuses(&statSP, &atkSP, atk.Bonuses)

	atkCopy := *atk
	return atkCopy.applyStatPoints(newStats)
}

func getModifierFunctions(field string, statPoints *FullStatPoints, applyTo *FullStatPoints) (func() uint64, func(uint64)) {
	if applyTo == nil {
		applyTo = statPoints
	}

	switch field {
	case "HP":
		return func() uint64 { return statPoints.HP }, func(value uint64) { applyTo.HP += value }
	case "TP":
		return func() uint64 { return statPoints.Trample }, func(value uint64) { applyTo.Trample += value }
	case "AP":
		return func() uint64 { return statPoints.StatPoints.AP }, func(value uint64) { applyTo.StatPoints.AP += value }
	case "RP":
		return func() uint64 { return statPoints.StatPoints.RP }, func(value uint64) { applyTo.StatPoints.RP += value }
	case "MP":
		return func() uint64 { return statPoints.StatPoints.MP }, func(value uint64) { applyTo.StatPoints.MP += value }
	}

	return nil, nil
}

// ApplyBonuses Apply any bonuses to attack stats based on bonus string (TODO)
func ApplyBonuses(statPoints *FullStatPoints, applyTo *FullStatPoints, bonuses string) FullStatPoints {
	modifiedStats := *statPoints

	fieldRE := regexp.MustCompile("^([HARMT]P)")
	modstringRE := regexp.MustCompile("([+-])([0-9]+)([%]?)([HARMT]P)?")

	for _, modifier := range strings.Split(bonuses, ";") {
		modField := fieldRE.FindString(modifier)
		modifiers := modstringRE.FindAllStringSubmatch(modifier, -1)

		for _, modifier := range modifiers {
			operand := modifier[1]
			numberAsString := modifier[2]
			optionalPercentage := modifier[3]
			fromField := modifier[4]

			number, _ := strconv.Atoi(numberAsString)

			multiplier := 1
			if operand == "-" {
				multiplier = -1
			}

			if optionalPercentage != "" {
				if fromField == "" {
					fromField = modField
				}

				getter, _ := getModifierFunctions(fromField, &modifiedStats, applyTo)
				_, setter := getModifierFunctions(modField, &modifiedStats, applyTo)

				value := float64(getter()) * (float64(number) / 100.0)
				setter(uint64(float64(multiplier) * value))
			} else {
				if fromField == "" && numberAsString != "" {
					_, setter := getModifierFunctions(modField, &modifiedStats, applyTo)
					setter(uint64(int(number) * multiplier))
				} else if fromField != "" && numberAsString == "" {
					_, setter := getModifierFunctions(modField, &modifiedStats, applyTo)
					othergetter, _ := getModifierFunctions(fromField, &modifiedStats, applyTo)
					setter(uint64(int(othergetter()) * multiplier))
				} else {
					log.Printf("Illegal modifier: %v", modifier)
				}
			}
		}
	}

	return modifiedStats
}
