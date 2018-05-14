package mud

import "fmt"

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

// StatPoints is a passable struct for managing attack/defense calculations
type StatPoints struct {
	AP uint64
	RP uint64
	MP uint64
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
	ID       string   `json:",omitempty"`
	Name     string   `json:""`
	Accuracy byte     `json:""`
	MP       uint64   `json:""`
	AP       uint64   `json:""`
	RP       uint64   `json:""`
	Trample  uint64   `json:""`
	Bonuses  string   `json:""`
	Effects  []string `json:""`
	Charge   int64    `json:""` // In Seconds
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

// ApplyBonuses Apply any bonuses to attack stats based on bonus string (TODO)
func (atk *Attack) ApplyBonuses(statPoints *StatPoints) Attack {
	return *atk
}
