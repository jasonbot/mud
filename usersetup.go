package mud

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/mgutz/ansi"

	"github.com/ahmetb/go-cursor"

	"github.com/gliderlabs/ssh"
)

type setMapThing struct {
	value byte
	name  string
}

var primaryStrengthArray = []setMapThing{
	{value: MELEEPRIMARY, name: "1: Melee"},
	{value: RANGEPRIMARY, name: "2: Range"},
	{value: MAGICPRIMARY, name: "3: Magic"},
}

var secondaryStrengthArray = []setMapThing{
	{value: MELEESECONDARY, name: "Q: Melee"},
	{value: RANGESECONDARY, name: "W: Range"},
	{value: MAGICSECONDARY, name: "E: Magic"},
}

var primarySkillArray = []setMapThing{
	{value: PEOPLEPRIMARY, name: "A: Melee"},
	{value: PLACESPRIMARY, name: "S: Range"},
	{value: THINGSPRIMARY, name: "D: Magic"},
}

var secondarySkillArray = []setMapThing{
	{value: PEOPLESECONDARY, name: "Z: Melee"},
	{value: PLACESSECONDARY, name: "X: Range"},
	{value: THINGSSECONDARY, name: "C: Magic"},
}

func renderChoices(selected byte, items []setMapThing) string {
	unselectedf := ansi.ColorFunc("white")
	selectedf := ansi.ColorFunc("black:white")

	retstring := ""
	for index, value := range items {
		if index > 0 {
			retstring += "  ⌑  "
		}

		if value.value == selected {
			retstring += selectedf(value.name)
		} else {
			retstring += unselectedf(value.name)
		}
	}

	return retstring + ansi.ColorCode("reset")
}

func renderSetup(session ssh.Session, user User) {
	primarystrength, secondarystrength := user.Strengths()
	primaryskill, secondaryskill := user.Skills()

	header := ansi.ColorFunc("white+b:black")
	io.WriteString(session, cursor.ClearEntireScreen()+cursor.MoveUpperLeft(1))
	io.WriteString(session, fmt.Sprintf("Please set up your character, %v.\n\n", user.Username()))

	io.WriteString(session, header("Strength:"))
	io.WriteString(session, "\n")
	io.WriteString(session, "      Primary: "+renderChoices(primarystrength, primaryStrengthArray))
	io.WriteString(session, "\n")
	io.WriteString(session, "    Secondary: "+renderChoices(secondarystrength, secondaryStrengthArray))
	io.WriteString(session, "\n\n")

	io.WriteString(session, header("Skill:"))
	io.WriteString(session, "\n")
	io.WriteString(session, "      Primary: "+renderChoices(primaryskill, primarySkillArray))
	io.WriteString(session, "\n")
	io.WriteString(session, "    Secondary: "+renderChoices(secondaryskill, secondarySkillArray))
	io.WriteString(session, "\n\n")
	io.WriteString(session, "Press enter when you are finished.")
}

func setupSSHUser(ctx context.Context, cancel context.CancelFunc, done <-chan struct{}, session ssh.Session, user User, stringInput chan inputEvent) {
	user.SetClassInfo(MELEEPRIMARY | MELEESECONDARY | PEOPLEPRIMARY | PEOPLESECONDARY)

	renderSetup(session, user)

	for {
		select {
		case inputString := <-stringInput:
			primarystrength, secondarystrength := user.Strengths()
			primaryskill, secondaryskill := user.Skills()

			if inputString.err != nil {
				session.Close()
				continue
			}
			switch inputString.inputString {
			case "1":
				primarystrength = MELEEPRIMARY
			case "2":
				primarystrength = RANGEPRIMARY
			case "3":
				primarystrength = MAGICPRIMARY

			case "q":
				fallthrough
			case "Q":
				secondarystrength = MELEESECONDARY
			case "w":
				fallthrough
			case "W":
				secondarystrength = RANGESECONDARY
			case "e":
				fallthrough
			case "E":
				secondarystrength = MAGICSECONDARY

			case "a":
				fallthrough
			case "A":
				primaryskill = PEOPLEPRIMARY
			case "s":
				fallthrough
			case "S":
				primaryskill = PLACESPRIMARY
			case "d":
				fallthrough
			case "D":
				primaryskill = THINGSPRIMARY

			case "z":
				fallthrough
			case "Z":
				secondaryskill = PEOPLESECONDARY
			case "x":
				fallthrough
			case "X":
				secondaryskill = PLACESSECONDARY
			case "c":
				fallthrough
			case "C":
				secondaryskill = THINGSSECONDARY

			case "ESCAPE":
				session.Close()

			case "ENTER":
				user.Initialize(true)
				return
			}

			user.SetStrengths(primarystrength, secondarystrength)
			user.SetSkills(primaryskill, secondaryskill)
			renderSetup(session, user)

		case <-ctx.Done():
			cancel()
		case <-done:
			log.Printf("Disconnected setup %v", session.RemoteAddr())
			user.Log(LogItem{Message: fmt.Sprintf("Canceled player setup %v", time.Now().UTC().Format(time.RFC3339)), MessageType: MESSAGESYSTEM})
			session.Close()
			return
		}
	}
}
